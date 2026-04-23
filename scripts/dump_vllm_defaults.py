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


def _infer_usage_context_from_cmdline(cmdline: list[str]) -> str:
    # This script is PID-first for online serving processes.
    # Keep OPENAI_API_SERVER as the canonical context.
    _ = cmdline
    return "OPENAI_API_SERVER"


def _resolve_runtime_attention_backend(
    vllm_config: Any,
    errors: dict[str, str],
) -> dict[str, Any] | None:
    try:
        model_config = getattr(vllm_config, "model_config", None)
        cache_config = getattr(vllm_config, "cache_config", None)
        if model_config is None or cache_config is None:
            return None

        get_head_size = getattr(model_config, "get_head_size", None)
        if not callable(get_head_size):
            return None

        from vllm.config.vllm import set_current_vllm_config
        from vllm.v1.attention.selector import get_attn_backend

        with set_current_vllm_config(vllm_config):
            backend_cls = get_attn_backend(
                head_size=get_head_size(),
                dtype=getattr(model_config, "dtype", None),
                kv_cache_dtype=getattr(cache_config, "cache_dtype", None),
            )

        backend_name: Any = None
        get_name = getattr(backend_cls, "get_name", None)
        if callable(get_name):
            backend_name = get_name()
        if backend_name is None:
            backend_name = getattr(backend_cls, "__name__", None)

        cleaned_name = _clean_attention_backend(backend_name)
        if cleaned_name is None:
            return None

        return {
            "name": cleaned_name,
            "class": _callable_name(backend_cls),
            "module": getattr(backend_cls, "__module__", None),
            "source": "vllm.v1.attention.selector.get_attn_backend",
        }
    except Exception as exc:
        errors["effective.attention_backend_resolve"] = repr(exc)
        return None


def _serialize_effective_engine_config(
    vllm_config: Any,
    errors: dict[str, str],
) -> Any:
    serialized = _to_jsonable(vllm_config)
    if isinstance(serialized, dict):
        resolved_backend = _resolve_runtime_attention_backend(vllm_config, errors)
        if resolved_backend is not None:
            serialized["resolved_attention_backend"] = resolved_backend
    return serialized


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
        return _serialize_effective_engine_config(vllm_config, errors)
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
                return _serialize_effective_engine_config(cfg, errors)
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
                return _serialize_effective_engine_config(vllm_config, errors)
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


def _clean_attention_backend(value: Any) -> str | None:
    if value is None:
        return None
    cleaned = str(value).strip()
    if not cleaned:
        return None
    return cleaned


def _clean_string_value(value: Any) -> str | None:
    if value is None:
        return None
    cleaned = str(value).strip()
    if not cleaned:
        return None
    return cleaned


def _clean_served_model_name(value: Any) -> str | None:
    if isinstance(value, (list, tuple)):
        for item in value:
            cleaned = _clean_string_value(item)
            if cleaned is not None:
                return cleaned
        return None
    return _clean_string_value(value)


def _clean_dtype_value(value: Any) -> str | None:
    cleaned = _clean_string_value(value)
    if cleaned is None:
        return None
    if cleaned.startswith("torch."):
        return cleaned.split(".", 1)[1]
    return cleaned


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
        full_parallel_cfg: dict[str, Any] = {}
        fallback_engine_args: dict[str, Any] = effective_cfg.get("engine_args", {}) or {}
        derived_defaults: dict[str, Any] = effective_cfg.get(
            "derived_model_defaults", {}
        ) or {}
        resolved_attention_backend: dict[str, Any] = {}
    elif is_cfg_dict:
        full_model_cfg = effective_cfg.get("model_config", {}) or {}
        full_sched_cfg = effective_cfg.get("scheduler_config", {}) or {}
        full_cache_cfg = effective_cfg.get("cache_config", {}) or {}
        full_parallel_cfg = effective_cfg.get("parallel_config", {}) or {}
        fallback_engine_args = {}
        derived_defaults = {}
        resolved_attention_backend = (
            effective_cfg.get("resolved_attention_backend", {}) or {}
        )
    else:
        full_model_cfg = {}
        full_sched_cfg = {}
        full_cache_cfg = {}
        full_parallel_cfg = {}
        fallback_engine_args = {}
        derived_defaults = {}
        resolved_attention_backend = {}

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

    served_model_name, source = _pick_value(
        [
            (
                _clean_served_model_name(full_model_cfg.get("served_model_name")),
                "effective_engine_config.model_config.served_model_name",
            ),
            (
                _clean_served_model_name(fallback_engine_args.get("served_model_name")),
                "effective_engine_config.engine_args.served_model_name",
            ),
            (
                _clean_served_model_name(engine_args.get("served_model_name")),
                "engine_args_from_input.served_model_name",
            ),
            (
                _clean_served_model_name(parsed_cli.get("served_model_name")),
                "parsed_cli_from_input.served_model_name",
            ),
            (
                _clean_served_model_name(model),
                "effective_serve_parameters.model",
            ),
        ]
    )
    values["served_model_name"] = served_model_name
    sources["served_model_name"] = source

    tensor_parallel_size, source = _pick_value(
        [
            (
                full_parallel_cfg.get("tensor_parallel_size"),
                "effective_engine_config.parallel_config.tensor_parallel_size",
            ),
            (
                fallback_engine_args.get("tensor_parallel_size"),
                "effective_engine_config.engine_args.tensor_parallel_size",
            ),
            (
                engine_args.get("tensor_parallel_size"),
                "engine_args_from_input.tensor_parallel_size",
            ),
            (
                parsed_cli.get("tensor_parallel_size"),
                "parsed_cli_from_input.tensor_parallel_size",
            ),
        ]
    )
    values["tensor_parallel_size"] = tensor_parallel_size
    sources["tensor_parallel_size"] = source

    data_parallel_size, source = _pick_value(
        [
            (
                full_parallel_cfg.get("data_parallel_size"),
                "effective_engine_config.parallel_config.data_parallel_size",
            ),
            (
                fallback_engine_args.get("data_parallel_size"),
                "effective_engine_config.engine_args.data_parallel_size",
            ),
            (
                engine_args.get("data_parallel_size"),
                "engine_args_from_input.data_parallel_size",
            ),
            (
                parsed_cli.get("data_parallel_size"),
                "parsed_cli_from_input.data_parallel_size",
            ),
        ]
    )
    values["data_parallel_size"] = data_parallel_size
    sources["data_parallel_size"] = source

    pipeline_parallel_size, source = _pick_value(
        [
            (
                full_parallel_cfg.get("pipeline_parallel_size"),
                "effective_engine_config.parallel_config.pipeline_parallel_size",
            ),
            (
                fallback_engine_args.get("pipeline_parallel_size"),
                "effective_engine_config.engine_args.pipeline_parallel_size",
            ),
            (
                engine_args.get("pipeline_parallel_size"),
                "engine_args_from_input.pipeline_parallel_size",
            ),
            (
                parsed_cli.get("pipeline_parallel_size"),
                "parsed_cli_from_input.pipeline_parallel_size",
            ),
        ]
    )
    values["pipeline_parallel_size"] = pipeline_parallel_size
    sources["pipeline_parallel_size"] = source

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

    kv_cache_dtype, source = _pick_value(
        [
            (
                _clean_string_value(full_cache_cfg.get("cache_dtype")),
                "effective_engine_config.cache_config.cache_dtype",
            ),
            (
                _clean_string_value(fallback_engine_args.get("kv_cache_dtype")),
                "effective_engine_config.engine_args.kv_cache_dtype",
            ),
            (
                _clean_string_value(engine_args.get("kv_cache_dtype")),
                "engine_args_from_input.kv_cache_dtype",
            ),
            (
                _clean_string_value(parsed_cli.get("kv_cache_dtype")),
                "parsed_cli_from_input.kv_cache_dtype",
            ),
        ]
    )
    values["kv_cache_dtype"] = kv_cache_dtype
    sources["kv_cache_dtype"] = source

    enable_prefix_caching, source = _pick_value(
        [
            (
                full_cache_cfg.get("enable_prefix_caching"),
                "effective_engine_config.cache_config.enable_prefix_caching",
            ),
            (
                fallback_engine_args.get("enable_prefix_caching"),
                "effective_engine_config.engine_args.enable_prefix_caching",
            ),
            (
                engine_args.get("enable_prefix_caching"),
                "engine_args_from_input.enable_prefix_caching",
            ),
            (
                parsed_cli.get("enable_prefix_caching"),
                "parsed_cli_from_input.enable_prefix_caching",
            ),
        ]
    )
    values["enable_prefix_caching"] = enable_prefix_caching
    sources["enable_prefix_caching"] = source

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

    quantization, source = _pick_value(
        [
            (
                _clean_string_value(full_model_cfg.get("quantization")),
                "effective_engine_config.model_config.quantization",
            ),
            (
                _clean_string_value(fallback_engine_args.get("quantization")),
                "effective_engine_config.engine_args.quantization",
            ),
            (
                _clean_string_value(engine_args.get("quantization")),
                "engine_args_from_input.quantization",
            ),
            (
                _clean_string_value(parsed_cli.get("quantization")),
                "parsed_cli_from_input.quantization",
            ),
        ]
    )
    values["quantization"] = quantization
    sources["quantization"] = source

    dtype, source = _pick_value(
        [
            (
                _clean_dtype_value(full_model_cfg.get("dtype")),
                "effective_engine_config.model_config.dtype",
            ),
            (
                _clean_dtype_value(fallback_engine_args.get("dtype")),
                "effective_engine_config.engine_args.dtype",
            ),
            (
                _clean_dtype_value(engine_args.get("dtype")),
                "engine_args_from_input.dtype",
            ),
            (
                _clean_dtype_value(parsed_cli.get("dtype")),
                "parsed_cli_from_input.dtype",
            ),
        ]
    )
    values["dtype"] = dtype
    sources["dtype"] = source

    attention_backend, source = _pick_value(
        [
            (
                _clean_attention_backend(resolved_attention_backend.get("name")),
                "effective_engine_config.resolved_attention_backend.name",
            ),
            (
                _clean_attention_backend(full_model_cfg.get("attention_backend")),
                "effective_engine_config.model_config.attention_backend",
            ),
            (
                _clean_attention_backend(full_model_cfg.get("attn_backend")),
                "effective_engine_config.model_config.attn_backend",
            ),
            (
                _clean_attention_backend(fallback_engine_args.get("attention_backend")),
                "effective_engine_config.engine_args.attention_backend",
            ),
            (
                _clean_attention_backend(fallback_engine_args.get("attn_backend")),
                "effective_engine_config.engine_args.attn_backend",
            ),
            (
                _clean_attention_backend(engine_args.get("attention_backend")),
                "engine_args_from_input.attention_backend",
            ),
            (
                _clean_attention_backend(engine_args.get("attn_backend")),
                "engine_args_from_input.attn_backend",
            ),
            (
                _clean_attention_backend(parsed_cli.get("attention_backend")),
                "parsed_cli_from_input.attention_backend",
            ),
            (
                _clean_attention_backend(os.environ.get("VLLM_ATTENTION_BACKEND")),
                "pid_process.environ.VLLM_ATTENTION_BACKEND",
            ),
            ("default", "vllm_default"),
        ]
    )
    values["attention_backend"] = attention_backend
    sources["attention_backend"] = source
    if resolved_attention_backend:
        values["attention_backend_details"] = resolved_attention_backend

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
    pid_process: dict[str, Any] | None = None
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
        if not inferred_args:
            errors["pid.inferred_cli_args"] = (
                "Could not infer vLLM CLI args from /proc/<pid>/cmdline."
            )
    except Exception as exc:
        errors["pid.read"] = repr(exc)

    if pid_process and pid_process["inferred_vllm_cli_args"]:
        inferred_args = pid_process["inferred_vllm_cli_args"]
        parsed_cli_from_input, engine_args_from_input = _parse_vllm_cli_input(
            inferred_args,
            errors,
        )

    result: dict[str, Any] = {
        "metadata": _build_metadata(args),
        "cli_defaults": _extract_cli_defaults(errors),
        "config_defaults": _extract_config_defaults(errors),
        "engine_args_defaults": _extract_engine_args_defaults(errors),
    }

    result["input_source"] = input_source
    if pid_process is not None:
        result["pid_process"] = pid_process

    if parsed_cli_from_input is not None:
        result["parsed_cli_from_input"] = parsed_cli_from_input
    if engine_args_from_input is not None:
        result["engine_args_from_input"] = _to_jsonable(engine_args_from_input)

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
