#!/usr/bin/env python
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright contributors to the vLLM project
"""Dump vLLM defaults to JSON for cross-version comparison.

This script is designed to run against an *installed* vLLM package
(`uv pip install vllm==...`) and does not require a local source checkout.

What it extracts:
1. CLI parser defaults (OpenAI/serve parser + AsyncEngineArgs parser)
2. Declared dataclass/config defaults from `vllm.config.*`
3. Declared defaults for `EngineArgs` / `AsyncEngineArgs`
4. Declared + normalized defaults for `SamplingParams` / `PoolingParams`
5. Effective runtime config via `AsyncEngineArgs.create_engine_config`
   (may require model/config resolution and environment support)
"""

from __future__ import annotations

import argparse
import dataclasses
import enum
import importlib
import inspect
import json
import os
import sys
from pathlib import Path
from typing import Any


NO_DEFAULT = "__NO_DEFAULT__"
RECURSIVE = "__RECURSIVE__"


def _optional_import(module_name: str) -> Any | None:
    try:
        return importlib.import_module(module_name)
    except Exception:
        return None


def _callable_name(obj: Any) -> str:
    module = getattr(obj, "__module__", "")
    qualname = getattr(obj, "__qualname__", getattr(obj, "__name__", repr(obj)))
    if module:
        return f"{module}.{qualname}"
    return str(qualname)


def _to_jsonable(
    obj: Any,
    *,
    _seen: set[int] | None = None,
    _depth: int = 0,
    _max_depth: int = 8,
) -> Any:
    if _seen is None:
        _seen = set()

    if _depth > _max_depth:
        return f"<max_depth:{type(obj).__name__}>"

    if obj is None or isinstance(obj, (bool, int, float, str)):
        return obj

    if isinstance(obj, enum.Enum):
        return {"enum": _callable_name(type(obj)), "name": obj.name, "value": obj.value}

    if isinstance(obj, Path):
        return str(obj)

    if isinstance(obj, argparse.Namespace):
        return _to_jsonable(vars(obj), _seen=_seen, _depth=_depth + 1)

    if inspect.isclass(obj) or callable(obj):
        return _callable_name(obj)

    oid = id(obj)
    if oid in _seen:
        return RECURSIVE
    _seen.add(oid)

    try:
        if dataclasses.is_dataclass(obj) and not inspect.isclass(obj):
            out: dict[str, Any] = {}
            for field in dataclasses.fields(obj):
                out[field.name] = _to_jsonable(
                    getattr(obj, field.name), _seen=_seen, _depth=_depth + 1
                )
            return out

        if hasattr(obj, "model_dump") and callable(obj.model_dump):
            try:
                return _to_jsonable(
                    obj.model_dump(), _seen=_seen, _depth=_depth + 1
                )
            except Exception:
                pass

        if isinstance(obj, dict):
            out_dict: dict[str, Any] = {}
            for key, value in obj.items():
                out_dict[str(key)] = _to_jsonable(
                    value, _seen=_seen, _depth=_depth + 1
                )
            return out_dict

        if isinstance(obj, (list, tuple, set, frozenset)):
            return [_to_jsonable(v, _seen=_seen, _depth=_depth + 1) for v in obj]

        struct_fields = getattr(type(obj), "__struct_fields__", None)
        if struct_fields and not inspect.isclass(obj):
            return {
                str(name): _to_jsonable(getattr(obj, name), _seen=_seen, _depth=_depth + 1)
                for name in struct_fields
            }

        return repr(obj)
    finally:
        _seen.discard(oid)


def _resolve_field_default(field: dataclasses.Field[Any]) -> tuple[bool, Any]:
    pydantic_field_info = None
    pydantic_undefined = None

    pydantic_fields_mod = _optional_import("pydantic.fields")
    if pydantic_fields_mod is not None:
        pydantic_field_info = getattr(pydantic_fields_mod, "FieldInfo", None)

    pydantic_core_mod = _optional_import("pydantic_core")
    if pydantic_core_mod is not None:
        pydantic_undefined = getattr(pydantic_core_mod, "PydanticUndefined", None)

    if field.default is not dataclasses.MISSING:
        default = field.default

        if pydantic_field_info is not None and isinstance(default, pydantic_field_info):
            default_factory = getattr(default, "default_factory", None)
            if default_factory is not None:
                try:
                    return True, default_factory()
                except Exception as exc:
                    return True, f"<default_factory_error:{exc}>"

            explicit_default = getattr(default, "default", dataclasses.MISSING)
            if explicit_default is dataclasses.MISSING:
                return False, NO_DEFAULT
            if pydantic_undefined is not None and explicit_default is pydantic_undefined:
                return False, NO_DEFAULT
            return True, explicit_default

        return True, default

    if field.default_factory is not dataclasses.MISSING:
        try:
            return True, field.default_factory()
        except Exception as exc:
            return True, f"<default_factory_error:{exc}>"

    return False, NO_DEFAULT


def _extract_dataclass_defaults(cls: type[Any]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for field in dataclasses.fields(cls):
        has_default, default_value = _resolve_field_default(field)
        out[field.name] = _to_jsonable(default_value) if has_default else NO_DEFAULT
    return out


def _extract_msgspec_declared_defaults(cls: type[Any]) -> dict[str, Any]:
    struct_fields = list(getattr(cls, "__struct_fields__", []))
    struct_defaults = list(getattr(cls, "__struct_defaults__", []))

    out = {str(name): NO_DEFAULT for name in struct_fields}
    if not struct_defaults:
        return out

    first_default_index = len(struct_fields) - len(struct_defaults)
    for idx, default in enumerate(struct_defaults):
        field_name = str(struct_fields[first_default_index + idx])
        out[field_name] = _to_jsonable(default)
    return out


def _extract_parser_defaults(parser: argparse.ArgumentParser) -> dict[str, Any]:
    parsed = parser.parse_args([])
    return _to_jsonable(vars(parsed))


def _extract_cli_defaults(errors: dict[str, str]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    argparse_utils = _optional_import("vllm.utils.argparse_utils")
    parser_cls = argparse.ArgumentParser
    if argparse_utils is not None:
        parser_cls = getattr(argparse_utils, "FlexibleArgumentParser", argparse.ArgumentParser)

    cli_args_mod = _optional_import("vllm.entrypoints.openai.cli_args")
    if cli_args_mod is not None and hasattr(cli_args_mod, "make_arg_parser"):
        try:
            parser = parser_cls(prog="vllm serve")
            parser = cli_args_mod.make_arg_parser(parser)
            out["serve_make_arg_parser"] = _extract_parser_defaults(parser)
        except Exception as exc:
            errors["cli.serve_make_arg_parser"] = repr(exc)

    arg_utils_mod = _optional_import("vllm.engine.arg_utils")
    if arg_utils_mod is not None and hasattr(arg_utils_mod, "AsyncEngineArgs"):
        try:
            parser = parser_cls(prog="vllm-async-engine")
            parser = arg_utils_mod.AsyncEngineArgs.add_cli_args(parser)
            out["async_engine_add_cli_args"] = _extract_parser_defaults(parser)
        except Exception as exc:
            errors["cli.async_engine_add_cli_args"] = repr(exc)

    return out


def _extract_config_defaults(errors: dict[str, str]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    config_mod = _optional_import("vllm.config")
    if config_mod is None:
        errors["config.module"] = "Could not import vllm.config"
        return out

    arg_utils_mod = _optional_import("vllm.engine.arg_utils")
    get_kwargs = getattr(arg_utils_mod, "get_kwargs", None) if arg_utils_mod else None

    for name in sorted(dir(config_mod)):
        if not name.endswith("Config"):
            continue
        cls = getattr(config_mod, name, None)
        if not inspect.isclass(cls):
            continue
        if cls.__module__ != "vllm.config" and not cls.__module__.startswith("vllm.config."):
            continue
        if not dataclasses.is_dataclass(cls):
            continue

        try:
            if callable(get_kwargs):
                kwargs = get_kwargs(cls)
                out[name] = _to_jsonable({k: v.get("default", NO_DEFAULT) for k, v in kwargs.items()})
            else:
                out[name] = _extract_dataclass_defaults(cls)
        except Exception:
            # Fallback to direct dataclass extraction if get_kwargs fails.
            try:
                out[name] = _extract_dataclass_defaults(cls)
            except Exception as exc:
                errors[f"config.{name}"] = repr(exc)

    return out


def _extract_engine_args_defaults(errors: dict[str, str]) -> dict[str, Any]:
    out: dict[str, Any] = {}
    arg_utils_mod = _optional_import("vllm.engine.arg_utils")
    if arg_utils_mod is None:
        errors["engine_args.module"] = "Could not import vllm.engine.arg_utils"
        return out

    for cls_name in ("EngineArgs", "AsyncEngineArgs"):
        cls = getattr(arg_utils_mod, cls_name, None)
        if cls is None:
            continue
        try:
            out[cls_name] = _extract_dataclass_defaults(cls)
        except Exception as exc:
            errors[f"engine_args.{cls_name}"] = repr(exc)
    return out


def _extract_request_param_defaults(errors: dict[str, str]) -> dict[str, Any]:
    out: dict[str, Any] = {}

    sampling_mod = _optional_import("vllm.sampling_params")
    if sampling_mod is not None and hasattr(sampling_mod, "SamplingParams"):
        sampling_cls = sampling_mod.SamplingParams
        try:
            declared = _extract_msgspec_declared_defaults(sampling_cls)
            out["SamplingParams_declared"] = declared
        except Exception as exc:
            errors["request.SamplingParams_declared"] = repr(exc)
        try:
            normalized = sampling_cls()
            out["SamplingParams_normalized_instance"] = _to_jsonable(normalized)
        except Exception as exc:
            errors["request.SamplingParams_normalized_instance"] = repr(exc)
    else:
        errors["request.SamplingParams"] = "Could not import vllm.sampling_params.SamplingParams"

    pooling_mod = _optional_import("vllm.pooling_params")
    if pooling_mod is not None and hasattr(pooling_mod, "PoolingParams"):
        pooling_cls = pooling_mod.PoolingParams
        try:
            declared = _extract_msgspec_declared_defaults(pooling_cls)
            out["PoolingParams_declared"] = declared
        except Exception as exc:
            errors["request.PoolingParams_declared"] = repr(exc)
        try:
            normalized = pooling_cls()
            out["PoolingParams_normalized_instance"] = _to_jsonable(normalized)
        except Exception as exc:
            errors["request.PoolingParams_normalized_instance"] = repr(exc)
    else:
        errors["request.PoolingParams"] = "Could not import vllm.pooling_params.PoolingParams"

    return out


def _resolve_usage_context(raw: str) -> Any | None:
    usage_mod = _optional_import("vllm.usage.usage_lib")
    if usage_mod is None:
        return None

    usage_cls = getattr(usage_mod, "UsageContext", None)
    if usage_cls is None:
        return None

    if hasattr(usage_cls, "__members__"):
        members = getattr(usage_cls, "__members__", {})
        if raw in members:
            return members[raw]

    for member in usage_cls:  # type: ignore[operator]
        if getattr(member, "name", None) == raw:
            return member
        if getattr(member, "value", None) == raw:
            return member

    return None


def _read_proc_null_separated(path: Path) -> list[str]:
    data = path.read_bytes()
    return [
        part.decode("utf-8", errors="replace")
        for part in data.split(b"\x00")
        if part
    ]


def _read_pid_cmdline(pid: int) -> list[str]:
    return _read_proc_null_separated(Path(f"/proc/{pid}/cmdline"))


def _read_pid_environ(pid: int) -> dict[str, str]:
    entries = _read_proc_null_separated(Path(f"/proc/{pid}/environ"))
    out: dict[str, str] = {}
    for entry in entries:
        if "=" not in entry:
            continue
        key, value = entry.split("=", 1)
        out[key] = value
    return out


def _infer_vllm_cli_args_from_cmdline(cmdline: list[str]) -> list[str]:
    if not cmdline:
        return []

    argv = cmdline[1:]

    if "-m" in argv:
        mod_idx = argv.index("-m")
        if mod_idx + 1 < len(argv):
            module = argv[mod_idx + 1]
            if module == "vllm" or module.startswith("vllm."):
                argv = argv[mod_idx + 2 :]

    if argv and os.path.basename(argv[0]) == "vllm":
        argv = argv[1:]

    if argv and argv[0] == "serve":
        argv = argv[1:]

    return argv


def _is_sensitive_env_key(key: str) -> bool:
    upper = key.upper()
    sensitive_markers = (
        "TOKEN",
        "SECRET",
        "PASSWORD",
        "PASSWD",
        "API_KEY",
        "AUTH",
        "COOKIE",
        "CREDENTIAL",
        "PRIVATE_KEY",
    )
    return any(marker in upper for marker in sensitive_markers)


def _redact_env(env_map: dict[str, str]) -> dict[str, str]:
    out: dict[str, str] = {}
    for key, value in env_map.items():
        out[key] = "<redacted>" if _is_sensitive_env_key(key) else value
    return out


def _apply_pid_environment(env_map: dict[str, str]) -> None:
    for key, value in env_map.items():
        os.environ[key] = value


def _force_platform_if_needed(
    pid_environ: dict[str, str],
    errors: dict[str, str],
) -> None:
    try:
        from vllm import platforms

        current = platforms.current_platform
        if getattr(current, "device_type", ""):
            return

        target = pid_environ.get("VLLM_TARGET_DEVICE", "cuda").lower()
        if target == "cuda":
            from vllm.platforms.cuda import CudaPlatform

            platforms.current_platform = CudaPlatform()
            return
        if target == "cpu":
            from vllm.platforms.cpu import CpuPlatform

            platforms.current_platform = CpuPlatform()
            return
        if target == "xpu":
            from vllm.platforms.xpu import XPUPlatform

            platforms.current_platform = XPUPlatform()
            return
    except Exception as exc:
        errors["platform.force"] = repr(exc)


def _parse_vllm_cli_input(
    raw_cli_args: list[str],
    errors: dict[str, str],
) -> tuple[dict[str, Any] | None, Any | None]:
    cli_args_mod = _optional_import("vllm.entrypoints.openai.cli_args")
    argparse_utils = _optional_import("vllm.utils.argparse_utils")
    arg_utils_mod = _optional_import("vllm.engine.arg_utils")

    if (
        cli_args_mod is None
        or not hasattr(cli_args_mod, "make_arg_parser")
        or arg_utils_mod is None
        or not hasattr(arg_utils_mod, "AsyncEngineArgs")
    ):
        errors["input_cli_args"] = "Required vLLM modules unavailable."
        return None, None

    parser_cls = argparse.ArgumentParser
    if argparse_utils is not None:
        parser_cls = getattr(
            argparse_utils, "FlexibleArgumentParser", argparse.ArgumentParser
        )

    try:
        parser = parser_cls(prog="vllm serve")
        parser = cli_args_mod.make_arg_parser(parser)
        parsed_ns = parser.parse_args(raw_cli_args)
        parsed_cli = _to_jsonable(vars(parsed_ns))
        engine_args_obj = arg_utils_mod.AsyncEngineArgs.from_cli_args(parsed_ns)
        return parsed_cli, engine_args_obj
    except Exception as exc:
        errors["input_cli_args_parse"] = repr(exc)
        return None, None


def _cli_key_covered_in_parsed(flag: str, parsed_cli: dict[str, Any]) -> bool:
    key = flag.lstrip("-").replace("-", "_")
    if key.startswith("no_"):
        key = key[3:]
    return key in parsed_cli


def _validate_cli_arg_coverage(
    raw_cli_args: list[str], parsed_cli: dict[str, Any] | None
) -> dict[str, Any]:
    if parsed_cli is None:
        return {
            "raw_flags": [],
            "covered_flags": [],
            "missing_flags": [],
            "all_flags_covered": False,
        }

    raw_flags = [arg for arg in raw_cli_args if arg.startswith("--")]
    covered_flags = [flag for flag in raw_flags if _cli_key_covered_in_parsed(flag, parsed_cli)]
    missing_flags = [flag for flag in raw_flags if flag not in covered_flags]

    return {
        "raw_flags": raw_flags,
        "covered_flags": covered_flags,
        "missing_flags": missing_flags,
        "all_flags_covered": len(missing_flags) == 0,
    }


def _infer_usage_context_from_cmdline(cmdline: list[str]) -> str:
    # This script is PID-first for online serving processes.
    # Keep OPENAI_API_SERVER as the canonical context.
    _ = cmdline
    return "OPENAI_API_SERVER"


def _extract_effective_engine_config(
    *,
    engine_args_obj: Any | None,
    usage_context_name: str,
    errors: dict[str, str],
    warnings: dict[str, str],
) -> Any:
    if engine_args_obj is None:
        errors["effective.engine_args"] = (
            "No parsed engine args available from PID CLI arguments."
        )
        return None

    usage_ctx = _resolve_usage_context(usage_context_name)
    if usage_ctx is None:
        errors["effective.usage_context"] = (
            f"Unable to resolve UsageContext={usage_context_name!r}."
        )
        return None

    try:
        vllm_config = engine_args_obj.create_engine_config(
            usage_context=usage_ctx,
            headless=False,
        )
        return _to_jsonable(vllm_config)
    except Exception as exc:
        message = repr(exc)

        def _build_engine_args_fallback(
            warn_message: str,
        ) -> dict[str, Any]:
            fallback: dict[str, Any] = {
                "_fallback": "engine_args_from_input",
                "_usage_context": usage_context_name,
                "engine_args": _to_jsonable(engine_args_obj),
            }
            try:
                from vllm import platforms
                import vllm.engine.arg_utils as engine_arg_utils
                from vllm.platforms.cpu import CpuPlatform

                os.environ["VLLM_TARGET_DEVICE"] = "cpu"
                cpu_platform = CpuPlatform()
                platforms.current_platform = cpu_platform
                engine_arg_utils.current_platform = cpu_platform
                # Retry using a fresh EngineArgs object to avoid any partial
                # mutation from the failed primary path.
                import dataclasses

                cls = type(engine_args_obj)
                kwargs = {
                    field.name: getattr(engine_args_obj, field.name)
                    for field in dataclasses.fields(cls)
                    if field.init
                }
                fresh_engine_args = cls(**kwargs)
                cfg = fresh_engine_args.create_engine_config(
                    usage_context=usage_ctx,
                    headless=False,
                )
                warnings["effective.create_engine_config"] = (
                    warn_message
                    + " Recovered by rebuilding EngineArgs under forced CPU."
                )
                return _to_jsonable(cfg)
            except Exception as derive_exc:
                warnings["effective.derive_model_defaults"] = (
                    "Failed to recover effective config in fallback mode: "
                    f"{repr(derive_exc)}"
                )
            warnings["effective.create_engine_config"] = warn_message
            return fallback

        if "No CUDA GPUs are available" in message:
            try:
                from vllm import platforms
                from vllm.engine import arg_utils as engine_arg_utils
                from vllm.platforms.cpu import CpuPlatform

                os.environ["VLLM_TARGET_DEVICE"] = "cpu"
                cpu_platform = CpuPlatform()
                platforms.current_platform = cpu_platform
                engine_arg_utils.current_platform = cpu_platform
                vllm_config = engine_args_obj.create_engine_config(
                    usage_context=usage_ctx,
                    headless=False,
                )
                warnings["effective.create_engine_config"] = (
                    "Fell back to CPU platform because CUDA GPUs were not "
                    "visible in this exec context."
                )
                return _to_jsonable(vllm_config)
            except Exception as retry_exc:
                warn_message = (
                    "Could not materialize VllmConfig in this exec context "
                    f"(primary={message}; retry_cpu_fallback={repr(retry_exc)}). "
                    "Falling back to parsed EngineArgs from PID cmdline."
                )
                return _build_engine_args_fallback(warn_message)

        warnings["effective.create_engine_config"] = (
            "create_engine_config failed; falling back to parsed EngineArgs. "
            f"reason={message}"
        )
        return _build_engine_args_fallback(
            f"create_engine_config fallback: {message}"
        )


def _pick_value(
    candidates: list[tuple[Any, str]],
) -> tuple[Any, str | None]:
    for value, source in candidates:
        if value is not None:
            return value, source
    return None, None


def _aggregate_effective_serve_parameters(
    *,
    parsed_cli: dict[str, Any] | None,
    engine_args: dict[str, Any] | None,
    effective_cfg: Any,
    usage_context: str,
) -> dict[str, Any]:
    parsed_cli = parsed_cli or {}
    engine_args = engine_args or {}

    is_cfg_dict = isinstance(effective_cfg, dict)
    fallback_mode = (
        effective_cfg.get("_fallback")
        if is_cfg_dict
        else None
    )

    if is_cfg_dict and fallback_mode:
        full_model_cfg: dict[str, Any] = {}
        full_sched_cfg: dict[str, Any] = {}
        full_cache_cfg: dict[str, Any] = {}
        fallback_engine_args: dict[str, Any] = effective_cfg.get("engine_args", {}) or {}
        derived_defaults: dict[str, Any] = effective_cfg.get(
            "derived_model_defaults", {}
        ) or {}
    elif is_cfg_dict:
        full_model_cfg = effective_cfg.get("model_config", {}) or {}
        full_sched_cfg = effective_cfg.get("scheduler_config", {}) or {}
        full_cache_cfg = effective_cfg.get("cache_config", {}) or {}
        fallback_engine_args = {}
        derived_defaults = {}
    else:
        full_model_cfg = {}
        full_sched_cfg = {}
        full_cache_cfg = {}
        fallback_engine_args = {}
        derived_defaults = {}

    values: dict[str, Any] = {}
    sources: dict[str, str | None] = {}

    model, source = _pick_value(
        [
            (full_model_cfg.get("model"), "effective_engine_config.model_config.model"),
            (derived_defaults.get("model"), "effective_engine_config.derived_model_defaults.model"),
            (fallback_engine_args.get("model"), "effective_engine_config.engine_args.model"),
            (engine_args.get("model"), "engine_args_from_input.model"),
            (parsed_cli.get("model"), "parsed_cli_from_input.model"),
        ]
    )
    values["model"] = model
    sources["model"] = source

    max_model_len, source = _pick_value(
        [
            (
                full_model_cfg.get("max_model_len"),
                "effective_engine_config.model_config.max_model_len",
            ),
            (
                derived_defaults.get("max_model_len"),
                "effective_engine_config.derived_model_defaults.max_model_len",
            ),
            (
                fallback_engine_args.get("max_model_len"),
                "effective_engine_config.engine_args.max_model_len",
            ),
            (engine_args.get("max_model_len"), "engine_args_from_input.max_model_len"),
            (parsed_cli.get("max_model_len"), "parsed_cli_from_input.max_model_len"),
        ]
    )
    values["max_model_len"] = max_model_len
    sources["max_model_len"] = source

    max_num_batched_tokens, source = _pick_value(
        [
            (
                full_sched_cfg.get("max_num_batched_tokens"),
                "effective_engine_config.scheduler_config.max_num_batched_tokens",
            ),
            (
                fallback_engine_args.get("max_num_batched_tokens"),
                "effective_engine_config.engine_args.max_num_batched_tokens",
            ),
            (
                engine_args.get("max_num_batched_tokens"),
                "engine_args_from_input.max_num_batched_tokens",
            ),
            (
                parsed_cli.get("max_num_batched_tokens"),
                "parsed_cli_from_input.max_num_batched_tokens",
            ),
        ]
    )
    values["max_num_batched_tokens"] = max_num_batched_tokens
    sources["max_num_batched_tokens"] = source

    max_num_seqs, source = _pick_value(
        [
            (
                full_sched_cfg.get("max_num_seqs"),
                "effective_engine_config.scheduler_config.max_num_seqs",
            ),
            (
                fallback_engine_args.get("max_num_seqs"),
                "effective_engine_config.engine_args.max_num_seqs",
            ),
            (engine_args.get("max_num_seqs"), "engine_args_from_input.max_num_seqs"),
            (parsed_cli.get("max_num_seqs"), "parsed_cli_from_input.max_num_seqs"),
        ]
    )
    values["max_num_seqs"] = max_num_seqs
    sources["max_num_seqs"] = source

    gpu_memory_utilization, source = _pick_value(
        [
            (
                full_cache_cfg.get("gpu_memory_utilization"),
                "effective_engine_config.cache_config.gpu_memory_utilization",
            ),
            (
                fallback_engine_args.get("gpu_memory_utilization"),
                "effective_engine_config.engine_args.gpu_memory_utilization",
            ),
            (
                engine_args.get("gpu_memory_utilization"),
                "engine_args_from_input.gpu_memory_utilization",
            ),
            (
                parsed_cli.get("gpu_memory_utilization"),
                "parsed_cli_from_input.gpu_memory_utilization",
            ),
        ]
    )
    values["gpu_memory_utilization"] = gpu_memory_utilization
    sources["gpu_memory_utilization"] = source

    enable_chunked_prefill, source = _pick_value(
        [
            (
                full_sched_cfg.get("enable_chunked_prefill"),
                "effective_engine_config.scheduler_config.enable_chunked_prefill",
            ),
            (
                fallback_engine_args.get("enable_chunked_prefill"),
                "effective_engine_config.engine_args.enable_chunked_prefill",
            ),
            (
                engine_args.get("enable_chunked_prefill"),
                "engine_args_from_input.enable_chunked_prefill",
            ),
            (
                parsed_cli.get("enable_chunked_prefill"),
                "parsed_cli_from_input.enable_chunked_prefill",
            ),
        ]
    )
    values["enable_chunked_prefill"] = enable_chunked_prefill
    sources["enable_chunked_prefill"] = source

    values["_usage_context"] = usage_context
    values["_effective_mode"] = (
        "fallback" if fallback_mode else "full_vllm_config"
    )
    values["_sources"] = sources
    return values


def _build_metadata(args: argparse.Namespace) -> dict[str, Any]:
    vllm_mod = _optional_import("vllm")
    version = getattr(vllm_mod, "__version__", None) if vllm_mod else None
    module_path = getattr(vllm_mod, "__file__", None) if vllm_mod else None

    return {
        "python": sys.version,
        "executable": sys.executable,
        "vllm_version": version,
        "vllm_module_path": module_path,
        "argv": sys.argv,
        "options": {
            "pid": args.pid,
            "redact_pid_env": args.redact_pid_env,
        },
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--out",
        type=str,
        default="-",
        help="Output path. Use '-' for stdout (default).",
    )
    parser.add_argument(
        "--indent",
        type=int,
        default=2,
        help="JSON indent (default: 2).",
    )
    parser.add_argument(
        "--pid",
        type=int,
        required=True,
        help="Linux process PID to introspect. Reads /proc/<pid>/cmdline and "
        "/proc/<pid>/environ, infers vLLM CLI args, and uses them as input.",
    )
    parser.add_argument(
        "--redact-pid-env",
        action=argparse.BooleanOptionalAction,
        default=True,
        help="Redact sensitive environment values in output for --pid mode "
        "(default: true).",
    )
    parser.add_argument(
        "--fail-on-error",
        action="store_true",
        help="Exit with non-zero status if any section fails.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    errors: dict[str, str] = {}
    warnings: dict[str, str] = {}
    parsed_cli_from_input: dict[str, Any] | None = None
    engine_args_from_input: Any | None = None
    cli_arg_coverage: dict[str, Any] | None = None
    pid_process: dict[str, Any] | None = None
    raw_cli_args_from_input: list[str] = []
    input_source: str = "pid"
    inferred_usage_context = "OPENAI_API_SERVER"

    try:
        pid_cmdline = _read_pid_cmdline(args.pid)
        pid_environ = _read_pid_environ(args.pid)
        inferred_args = _infer_vllm_cli_args_from_cmdline(pid_cmdline)
        inferred_usage_context = _infer_usage_context_from_cmdline(pid_cmdline)

        _apply_pid_environment(pid_environ)
        _force_platform_if_needed(pid_environ, errors)

        pid_process = {
            "pid": args.pid,
            "cmdline": pid_cmdline,
            "inferred_vllm_cli_args": inferred_args,
            "inferred_usage_context": inferred_usage_context,
            "environ": _redact_env(pid_environ)
            if args.redact_pid_env
            else pid_environ,
            "pid_env_applied": True,
        }
        raw_cli_args_from_input = inferred_args
        if not inferred_args:
            errors["pid.inferred_cli_args"] = (
                "Could not infer vLLM CLI args from /proc/<pid>/cmdline."
            )
    except Exception as exc:
        errors["pid.read"] = repr(exc)

    if raw_cli_args_from_input:
        parsed_cli_from_input, engine_args_from_input = _parse_vllm_cli_input(
            raw_cli_args_from_input,
            errors,
        )
        cli_arg_coverage = _validate_cli_arg_coverage(
            raw_cli_args_from_input, parsed_cli_from_input
        )

    result: dict[str, Any] = {
        "metadata": _build_metadata(args),
        "cli_defaults": _extract_cli_defaults(errors),
        "config_defaults": _extract_config_defaults(errors),
        "engine_args_defaults": _extract_engine_args_defaults(errors),
        "request_defaults": _extract_request_param_defaults(errors),
    }

    result["input_source"] = input_source
    result["raw_cli_args_from_input"] = raw_cli_args_from_input
    if pid_process is not None:
        result["pid_process"] = pid_process

    if parsed_cli_from_input is not None:
        result["parsed_cli_from_input"] = parsed_cli_from_input
    if engine_args_from_input is not None:
        result["engine_args_from_input"] = _to_jsonable(engine_args_from_input)
    if cli_arg_coverage is not None:
        result["cli_arg_coverage"] = cli_arg_coverage

    result["effective_engine_config"] = _extract_effective_engine_config(
        engine_args_obj=engine_args_from_input,
        usage_context_name=inferred_usage_context,
        errors=errors,
        warnings=warnings,
    )
    result["effective_serve_parameters"] = _aggregate_effective_serve_parameters(
        parsed_cli=parsed_cli_from_input,
        engine_args=result.get("engine_args_from_input"),
        effective_cfg=result["effective_engine_config"],
        usage_context=inferred_usage_context,
    )

    if errors:
        result["errors"] = errors
    if warnings:
        result["warnings"] = warnings

    payload = json.dumps(result, indent=args.indent, sort_keys=True) + "\n"

    if args.out == "-":
        sys.stdout.write(payload)
    else:
        out_path = Path(args.out)
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(payload, encoding="utf-8")

    if args.fail_on_error and errors:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
