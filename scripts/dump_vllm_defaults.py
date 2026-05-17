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
4. Effective runtime config via `AsyncEngineArgs.create_engine_config`
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
import signal
import shlex
import sys
from contextlib import contextmanager
from pathlib import Path
from typing import Any


NO_DEFAULT = "__NO_DEFAULT__"
RECURSIVE = "__RECURSIVE__"


@dataclasses.dataclass
class _ProcSnapshot:
    pid: int
    ppid: int | None
    cmdline: list[str]
    cwd: str | None = None
    cgroup: str | None = None


@dataclasses.dataclass
class _CLIResolution:
    pid: int
    cmdline: list[str]
    inferred_args: list[str]
    warning: str | None
    source: str
    fallback_cmdline: list[str] | None = None


class _OperationTimeout(BaseException):
    """Timeout that should bypass broad third-party `except Exception` retries."""


def _optional_import(module_name: str) -> Any | None:
    try:
        return importlib.import_module(module_name)
    except SystemExit:
        return None
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
    try:
        parsed = parser.parse_args([])
    except SystemExit as exc:
        raise RuntimeError(f"parse_args exited: {exc}") from exc
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
        except SystemExit as exc:
            errors["cli.serve_make_arg_parser"] = repr(exc)
        except Exception as exc:
            errors["cli.serve_make_arg_parser"] = repr(exc)

    arg_utils_mod = _optional_import("vllm.engine.arg_utils")
    if arg_utils_mod is not None and hasattr(arg_utils_mod, "AsyncEngineArgs"):
        try:
            parser = parser_cls(prog="vllm-async-engine")
            parser = arg_utils_mod.AsyncEngineArgs.add_cli_args(parser)
            out["async_engine_add_cli_args"] = _extract_parser_defaults(parser)
        except SystemExit as exc:
            errors["cli.async_engine_add_cli_args"] = repr(exc)
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


def _read_pid_cwd(pid: int) -> str:
    return os.readlink(f"/proc/{pid}/cwd")


def _read_pid_ppid(pid: int) -> int | None:
    try:
        for line in Path(f"/proc/{pid}/status").read_text(
            encoding="utf-8",
            errors="replace",
        ).splitlines():
            if not line.startswith("PPid:"):
                continue
            raw = line.split(":", 1)[1].strip()
            return int(raw) if raw else None
    except Exception:
        return None
    return None


def _read_pid_cgroup(pid: int) -> str | None:
    try:
        return Path(f"/proc/{pid}/cgroup").read_text(
            encoding="utf-8",
            errors="replace",
        ).strip()
    except Exception:
        return None


def _read_proc_snapshots() -> dict[int, _ProcSnapshot]:
    snapshots: dict[int, _ProcSnapshot] = {}
    try:
        entries = list(Path("/proc").iterdir())
    except Exception:
        return snapshots

    for entry in entries:
        if not entry.name.isdigit():
            continue
        pid = int(entry.name)
        try:
            cmdline = _read_pid_cmdline(pid)
        except Exception:
            continue
        cwd: str | None = None
        try:
            cwd = _read_pid_cwd(pid)
        except Exception:
            pass
        snapshots[pid] = _ProcSnapshot(
            pid=pid,
            ppid=_read_pid_ppid(pid),
            cmdline=cmdline,
            cwd=cwd,
            cgroup=_read_pid_cgroup(pid),
        )
    return snapshots


def _apply_pid_cwd(cwd: str) -> str:
    os.chdir(cwd)
    return os.getcwd()


def _strip_optional_serve(argv: list[str]) -> list[str]:
    if argv and argv[0] == "serve":
        return argv[1:]
    return argv


def _is_python_executable(value: str) -> bool:
    executable = os.path.basename(value)
    return executable == "python" or executable.startswith("python")


def _python_module_args(argv: list[str]) -> list[str] | None:
    if len(argv) < 3:
        return None
    if not _is_python_executable(argv[0]):
        return None
    if argv[1] != "-m":
        return None
    module = argv[2]
    if module == "vllm" or module.startswith("vllm."):
        return _strip_optional_serve(argv[3:])
    return None


def _python_vllm_script_args(argv: list[str]) -> list[str] | None:
    if len(argv) < 2:
        return None
    if not _is_python_executable(argv[0]):
        return None
    if os.path.basename(argv[1]) != "vllm":
        return None
    return _strip_optional_serve(argv[2:])


def _direct_vllm_args(argv: list[str]) -> list[str] | None:
    if not argv:
        return None
    if os.path.basename(argv[0]) != "vllm":
        return None
    return _strip_optional_serve(argv[1:])


def _uv_run_args(argv: list[str]) -> list[str] | None:
    if len(argv) < 3:
        return None
    if os.path.basename(argv[0]) != "uv" or argv[1] != "run":
        return None
    # Only parse the exact wrapper forms we understand. uv run flags are not
    # reinterpreted here because that would become a second command parser.
    wrapped = argv[2:]
    return (
        _direct_vllm_args(wrapped)
        or _python_module_args(wrapped)
        or _python_vllm_script_args(wrapped)
    )


def _infer_vllm_cli_args_from_cmdline(
    cmdline: list[str],
) -> tuple[list[str], str | None]:
    if not cmdline:
        return [], "empty cmdline"

    for parser in (
        _direct_vllm_args,
        _python_module_args,
        _python_vllm_script_args,
        _uv_run_args,
    ):
        inferred = parser(cmdline)
        if inferred is not None:
            return inferred, None

    executable = os.path.basename(cmdline[0]) if cmdline else ""
    return [], f"unsupported vLLM launch shape: {executable or '<empty>'}"


def _parse_fallback_command_line(raw: str) -> list[str]:
    trimmed = raw.strip()
    if not trimmed:
        return []
    try:
        return shlex.split(trimmed)
    except ValueError:
        return trimmed.split()


def _serve_cli_candidates(
    snapshots: dict[int, _ProcSnapshot],
) -> dict[int, tuple[_ProcSnapshot, list[str]]]:
    out: dict[int, tuple[_ProcSnapshot, list[str]]] = {}
    for snapshot in snapshots.values():
        inferred, _ = _infer_vllm_cli_args_from_cmdline(snapshot.cmdline)
        if inferred:
            out[snapshot.pid] = (snapshot, inferred)
    return out


def _ancestor_pids(
    pid: int,
    snapshots: dict[int, _ProcSnapshot],
) -> list[int]:
    out: list[int] = []
    seen = {pid}
    current = snapshots.get(pid)
    while current is not None and current.ppid:
        ppid = current.ppid
        if ppid in seen:
            break
        seen.add(ppid)
        out.append(ppid)
        current = snapshots.get(ppid)
    return out


def _single_related_candidate(
    candidates: list[tuple[_ProcSnapshot, list[str]]],
) -> tuple[_ProcSnapshot, list[str]] | None:
    if len(candidates) != 1:
        return None
    return candidates[0]


def _resolve_vllm_cli_process(
    *,
    requested_pid: int,
    requested_cmdline: list[str],
    requested_cwd: str | None,
    fallback_command_line: str = "",
) -> _CLIResolution:
    inferred, warning = _infer_vllm_cli_args_from_cmdline(requested_cmdline)
    if inferred:
        return _CLIResolution(
            pid=requested_pid,
            cmdline=requested_cmdline,
            inferred_args=inferred,
            warning=None,
            source="requested_pid",
        )

    snapshots = _read_proc_snapshots()
    if requested_pid not in snapshots:
        snapshots[requested_pid] = _ProcSnapshot(
            pid=requested_pid,
            ppid=_read_pid_ppid(requested_pid),
            cmdline=requested_cmdline,
            cwd=requested_cwd,
            cgroup=_read_pid_cgroup(requested_pid),
        )
    elif requested_cwd and snapshots[requested_pid].cwd is None:
        snapshots[requested_pid].cwd = requested_cwd

    serve_candidates = _serve_cli_candidates(snapshots)
    for ancestor_pid in _ancestor_pids(requested_pid, snapshots):
        candidate = serve_candidates.get(ancestor_pid)
        if candidate is None:
            continue
        snapshot, candidate_args = candidate
        return _CLIResolution(
            pid=snapshot.pid,
            cmdline=snapshot.cmdline,
            inferred_args=candidate_args,
            warning=None,
            source="ancestor_process",
        )

    target = snapshots.get(requested_pid)
    target_cwd = requested_cwd or (target.cwd if target else None)
    if target_cwd:
        candidate = _single_related_candidate(
            [
                (snapshot, candidate_args)
                for snapshot, candidate_args in serve_candidates.values()
                if snapshot.cwd == target_cwd
            ]
        )
        if candidate is not None:
            snapshot, candidate_args = candidate
            return _CLIResolution(
                pid=snapshot.pid,
                cmdline=snapshot.cmdline,
                inferred_args=candidate_args,
                warning=None,
                source="same_cwd_process",
            )

    target_cgroup = target.cgroup if target else None
    if target_cgroup:
        candidate = _single_related_candidate(
            [
                (snapshot, candidate_args)
                for snapshot, candidate_args in serve_candidates.values()
                if snapshot.cgroup == target_cgroup
            ]
        )
        if candidate is not None:
            snapshot, candidate_args = candidate
            return _CLIResolution(
                pid=snapshot.pid,
                cmdline=snapshot.cmdline,
                inferred_args=candidate_args,
                warning=None,
                source="same_cgroup_process",
            )

    fallback_cmdline = _parse_fallback_command_line(fallback_command_line)
    if fallback_cmdline:
        fallback_args, fallback_warning = _infer_vllm_cli_args_from_cmdline(
            fallback_cmdline,
        )
        if fallback_args:
            return _CLIResolution(
                pid=requested_pid,
                cmdline=requested_cmdline,
                inferred_args=fallback_args,
                warning=None,
                source="fallback_command_line",
                fallback_cmdline=fallback_cmdline,
            )
        warning = (
            f"{warning}; fallback_command_line: {fallback_warning}"
            if warning and fallback_warning
            else warning or fallback_warning
        )

    only_candidate = _single_related_candidate(list(serve_candidates.values()))
    if only_candidate is not None:
        snapshot, candidate_args = only_candidate
        return _CLIResolution(
            pid=snapshot.pid,
            cmdline=snapshot.cmdline,
            inferred_args=candidate_args,
            warning=None,
            source="only_vllm_process",
        )

    return _CLIResolution(
        pid=requested_pid,
        cmdline=requested_cmdline,
        inferred_args=[],
        warning=warning,
        source="unresolved",
    )


def _cli_option_value(raw_cli_args: list[str], *names: str) -> str | None:
    name_set = set(names)
    prefixes = tuple(name + "=" for name in names)
    for idx, token in enumerate(raw_cli_args):
        if token in name_set:
            if idx + 1 < len(raw_cli_args):
                return raw_cli_args[idx + 1]
            return None
        for prefix in prefixes:
            if token.startswith(prefix):
                return token[len(prefix):]
    return None


def _model_arg_from_cli(raw_cli_args: list[str]) -> str | None:
    explicit = _cli_option_value(raw_cli_args, "--model", "--model-tag")
    if explicit is not None:
        return explicit
    positional_idx = _find_positional_model_arg_index(raw_cli_args)
    if positional_idx is None:
        return None
    return raw_cli_args[positional_idx]


def _looks_like_hf_repo_id(value: str) -> bool:
    cleaned = value.strip()
    if not cleaned or "/" not in cleaned:
        return False
    if cleaned.startswith(("/", "./", "../", "~")):
        return False
    if "://" in cleaned:
        return False
    return True


def _hf_cache_roots(pid_environ: dict[str, str]) -> list[Path]:
    roots: list[Path] = []

    def add(raw: str | None) -> None:
        if raw is None:
            return
        cleaned = raw.strip()
        if not cleaned:
            return
        path = Path(cleaned).expanduser()
        if path not in roots:
            roots.append(path)

    for key in (
        "HF_HUB_CACHE",
        "HUGGING_FACE_HUB_CACHE",
        "HUGGINGFACE_HUB_CACHE",
    ):
        add(pid_environ.get(key) or os.environ.get(key))

    hf_home = pid_environ.get("HF_HOME") or os.environ.get("HF_HOME")
    if hf_home:
        add(str(Path(hf_home).expanduser() / "hub"))

    xdg_cache = pid_environ.get("XDG_CACHE_HOME") or os.environ.get("XDG_CACHE_HOME")
    if xdg_cache:
        add(str(Path(xdg_cache).expanduser() / "huggingface" / "hub"))

    home = pid_environ.get("HOME") or os.environ.get("HOME")
    if home:
        add(str(Path(home).expanduser() / ".cache" / "huggingface" / "hub"))

    for key in ("TRANSFORMERS_CACHE",):
        add(pid_environ.get(key) or os.environ.get(key))

    return roots


def _cached_snapshot_from_repo_dir(
    repo_dir: Path,
    revision: str | None,
) -> tuple[Path | None, str | None]:
    snapshots_dir = repo_dir / "snapshots"
    if not snapshots_dir.is_dir():
        return None, None

    revision = revision.strip() if revision else ""
    if revision:
        ref_path = repo_dir / "refs" / revision
        try:
            commit = ref_path.read_text(encoding="utf-8").strip()
        except Exception:
            commit = ""
        if commit:
            candidate = snapshots_dir / commit
            if candidate.is_dir():
                return candidate, f"hf_cache.refs.{revision}"
        candidate = snapshots_dir / revision
        if candidate.is_dir():
            return candidate, "hf_cache.revision"

    ref_path = repo_dir / "refs" / "main"
    try:
        commit = ref_path.read_text(encoding="utf-8").strip()
    except Exception:
        commit = ""
    if commit:
        candidate = snapshots_dir / commit
        if candidate.is_dir():
            return candidate, "hf_cache.refs.main"

    snapshots = [path for path in snapshots_dir.iterdir() if path.is_dir()]
    if len(snapshots) == 1:
        return snapshots[0], "hf_cache.single_snapshot"

    return None, None


def _resolve_cached_hf_snapshot(
    model_id: str,
    revision: str | None,
    pid_environ: dict[str, str],
) -> tuple[str | None, str | None]:
    repo_dir_name = "models--" + model_id.replace("/", "--")
    for cache_root in _hf_cache_roots(pid_environ):
        snapshot, source = _cached_snapshot_from_repo_dir(
            cache_root / repo_dir_name,
            revision,
        )
        if snapshot is not None:
            return str(snapshot), source
    return None, None


def _auto_model_path_override_from_cache(
    raw_cli_args: list[str],
    pid_environ: dict[str, str],
) -> tuple[str | None, str | None]:
    model_id = _model_arg_from_cli(raw_cli_args)
    if model_id is None or not _looks_like_hf_repo_id(model_id):
        return None, None
    revision = _cli_option_value(raw_cli_args, "--revision")
    return _resolve_cached_hf_snapshot(model_id, revision, pid_environ)


def _override_vllm_model_arg(
    raw_cli_args: list[str],
    model_override: str,
) -> tuple[list[str], bool]:
    override = model_override.strip()
    if not override:
        return raw_cli_args, False

    out = list(raw_cli_args)
    for idx, token in enumerate(out):
        if token in {"--model", "--model-tag"}:
            if idx + 1 < len(out):
                out[idx + 1] = override
            else:
                out.append(override)
            return out, True
        for prefix in ("--model=", "--model-tag="):
            if token.startswith(prefix):
                out[idx] = prefix + override
                return out, True

    positional_idx = _find_positional_model_arg_index(out)
    if positional_idx is not None:
        out[positional_idx] = override
        return out, True

    return [override, *out], True


def _find_positional_model_arg_index(raw_cli_args: list[str]) -> int | None:
    boolean_flags = {
        "--async-scheduling",
        "--disable-log-requests",
        "--enable-auto-tool-choice",
        "--enable-prefix-caching",
        "--enforce-eager",
        "--trust-remote-code",
    }
    idx = 0
    while idx < len(raw_cli_args):
        token = raw_cli_args[idx].strip()
        if not token:
            idx += 1
            continue
        if token == "--":
            next_idx = idx + 1
            return next_idx if next_idx < len(raw_cli_args) else None
        if token.startswith("-"):
            if (
                token.startswith("--")
                and "=" not in token
                and token not in boolean_flags
                and idx + 1 < len(raw_cli_args)
                and not raw_cli_args[idx + 1].startswith("-")
            ):
                idx += 2
                continue
            idx += 1
            continue
        return idx
    return None


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


def _allowed_pid_env(key: str) -> bool:
    exact = {
        "CONDA_DEFAULT_ENV",
        "CONDA_PREFIX",
        "CUDA_VISIBLE_DEVICES",
        "HF_ENDPOINT",
        "HF_HOME",
        "HF_HUB_CACHE",
        "HF_HUB_OFFLINE",
        "HOME",
        "HUGGING_FACE_HUB_CACHE",
        "HUGGING_FACE_HUB_TOKEN",
        "HUGGINGFACE_HUB_CACHE",
        "LD_LIBRARY_PATH",
        "PATH",
        "PYTHONHOME",
        "PYTHONPATH",
        "TORCH_HOME",
        "TRANSFORMERS_CACHE",
        "TRANSFORMERS_OFFLINE",
        "VIRTUAL_ENV",
        "VLLM_ATTENTION_BACKEND",
        "VLLM_TARGET_DEVICE",
        "XDG_CACHE_HOME",
    }
    prefixes = (
        "CUDA_",
        "HF_",
        "HUGGING_FACE_",
        "HUGGINGFACE_",
        "NCCL_",
        "NVIDIA_",
        "PYTORCH_",
        "TORCH_",
        "TRANSFORMERS_",
        "VLLM_",
    )
    return key in exact or any(key.startswith(prefix) for prefix in prefixes)


def _filter_pid_environment(env_map: dict[str, str]) -> dict[str, str]:
    return {key: value for key, value in env_map.items() if _allowed_pid_env(key)}


def _apply_pid_environment(env_map: dict[str, str]) -> list[str]:
    filtered = _filter_pid_environment(env_map)
    for key, value in filtered.items():
        os.environ[key] = value
    _apply_pythonpath(filtered.get("PYTHONPATH", ""))
    return sorted(filtered)


def _apply_pythonpath(raw: str) -> None:
    for item in reversed(raw.split(os.pathsep)):
        path = item.strip()
        if not path or path in sys.path:
            continue
        sys.path.insert(0, path)


def _force_platform_if_needed(
    pid_environ: dict[str, str],
    errors: dict[str, str],
) -> None:
    try:
        from vllm import platforms

        current = platforms.current_platform
        if getattr(current, "device_type", ""):
            return

        target = pid_environ.get("VLLM_TARGET_DEVICE", "").strip().lower()
        if not target:
            return
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
    parse_timeout_seconds: float | None = None,
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

    parsed_cli: dict[str, Any] | None = None
    try:
        parser = parser_cls(prog="vllm serve")
        parser = cli_args_mod.make_arg_parser(parser)
        default_model = _parser_default_model(parser)
        parsed_ns = parser.parse_args(raw_cli_args)
        _normalize_parsed_model_alias(
            parsed_ns,
            raw_cli_args,
            default_model=default_model,
        )
        parsed_cli = _to_jsonable(vars(parsed_ns))
    except SystemExit as exc:
        errors["input_cli_args_parse"] = repr(exc)
        return None, None
    except Exception as exc:
        errors["input_cli_args_parse"] = repr(exc)
        return None, None

    try:
        with _time_limit(parse_timeout_seconds, "input CLI parsing"):
            engine_args_obj = arg_utils_mod.AsyncEngineArgs.from_cli_args(parsed_ns)
        return parsed_cli, engine_args_obj
    except _OperationTimeout as exc:
        errors["input_cli_args_parse"] = repr(exc)
        return parsed_cli, None
    except Exception as exc:
        errors["input_cli_args_parse"] = repr(exc)
        return parsed_cli, None


def _parser_default_model(parser: argparse.ArgumentParser) -> Any:
    try:
        with _suppress_argparse_stderr():
            parsed = parser.parse_args([])
    except SystemExit:
        return None
    except Exception:
        return None
    return getattr(parsed, "model", None)


@contextmanager
def _suppress_argparse_stderr():
    try:
        old_stderr = sys.stderr
        with open(os.devnull, "w", encoding="utf-8") as devnull:
            sys.stderr = devnull
            yield
    finally:
        sys.stderr = old_stderr


def _normalize_parsed_model_alias(
    parsed_ns: argparse.Namespace,
    raw_cli_args: list[str],
    *,
    default_model: Any,
) -> None:
    model_tag = getattr(parsed_ns, "model_tag", None)
    if model_tag is None:
        return
    if str(model_tag).strip() == "":
        return
    if _cli_option_value(raw_cli_args, "--model") is not None:
        return
    current_model = getattr(parsed_ns, "model", None)
    if current_model is None or current_model == default_model:
        setattr(parsed_ns, "model", model_tag)


def _infer_usage_context_from_cmdline(cmdline: list[str]) -> str:
    # This script is PID-first for online serving processes.
    # Keep OPENAI_API_SERVER as the canonical context.
    _ = cmdline
    return "OPENAI_API_SERVER"


@contextmanager
def _time_limit(seconds: float | None, label: str):
    if seconds is None or seconds <= 0 or not hasattr(signal, "setitimer"):
        yield
        return

    def handle_timeout(signum: int, frame: Any) -> None:
        _ = signum, frame
        raise _OperationTimeout(f"{label} timed out after {seconds:g}s")

    previous_handler = signal.getsignal(signal.SIGALRM)
    signal.signal(signal.SIGALRM, handle_timeout)
    previous_timer = signal.setitimer(signal.ITIMER_REAL, seconds)
    try:
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, previous_handler)
        if previous_timer[0] > 0:
            signal.setitimer(signal.ITIMER_REAL, previous_timer[0], previous_timer[1])


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
    effective_timeout_seconds: float | None = None,
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
        with _time_limit(effective_timeout_seconds, "create_engine_config"):
            vllm_config = engine_args_obj.create_engine_config(
                usage_context=usage_ctx,
                headless=True,
            )
        return _serialize_effective_engine_config(vllm_config, errors)
    except (Exception, _OperationTimeout) as exc:
        message = repr(exc)

        def _degraded_config_fallback(
            *,
            reason: str,
            cfg: Any,
        ) -> dict[str, Any]:
            return {
                "_fallback": "degraded_effective_config",
                "_usage_context": usage_context_name,
                "_degraded_reason": reason,
                "engine_args": _to_jsonable(engine_args_obj),
                "degraded_effective_config": _serialize_effective_engine_config(
                    cfg,
                    errors,
                ),
            }

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
                with _time_limit(
                    effective_timeout_seconds,
                    "fallback create_engine_config",
                ):
                    cfg = fresh_engine_args.create_engine_config(
                        usage_context=usage_ctx,
                        headless=True,
                    )
                warnings["effective.create_engine_config"] = (
                    warn_message
                    + " Recovered by rebuilding EngineArgs under forced CPU."
                )
                return _degraded_config_fallback(
                    reason="forced CPU fallback after primary effective config failed",
                    cfg=cfg,
                )
            except (Exception, _OperationTimeout) as derive_exc:
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
                with _time_limit(effective_timeout_seconds, "cpu create_engine_config"):
                    vllm_config = engine_args_obj.create_engine_config(
                        usage_context=usage_ctx,
                        headless=True,
                    )
                warnings["effective.create_engine_config"] = (
                    "Fell back to CPU platform because CUDA GPUs were not "
                    "visible in this exec context. CPU-derived config is "
                    "recorded as degraded evidence and is not treated as the "
                    "actual runtime config."
                )
                return _degraded_config_fallback(
                    reason="forced CPU fallback because CUDA was unavailable",
                    cfg=vllm_config,
                )
            except (Exception, _OperationTimeout) as retry_exc:
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


def _requested_model_candidates(
    parsed_cli: dict[str, Any],
    engine_args: dict[str, Any],
) -> list[tuple[Any, str]]:
    return [
        (parsed_cli.get("model_tag"), "parsed_cli_from_input.model_tag"),
        (engine_args.get("model_tag"), "engine_args_from_input.model_tag"),
        (parsed_cli.get("model"), "parsed_cli_from_input.model"),
        (engine_args.get("model"), "engine_args_from_input.model"),
    ]


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

    def add_effective_value(
        name: str,
        candidates: list[tuple[Any, str]],
    ) -> None:
        value, source = _pick_value(candidates)
        values[name] = value
        sources[name] = source

    model, source = _pick_value(
        _requested_model_candidates(parsed_cli, engine_args)
        + [
            (full_model_cfg.get("model"), "effective_engine_config.model_config.model"),
            (
                derived_defaults.get("model"),
                "effective_engine_config.derived_model_defaults.model",
            ),
            (
                fallback_engine_args.get("model"),
                "effective_engine_config.engine_args.model",
            ),
        ]
    )
    values["model"] = model
    sources["model"] = source

    served_model_name, source = _pick_value(
        [
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
            (
                _clean_served_model_name(full_model_cfg.get("served_model_name")),
                "effective_engine_config.model_config.served_model_name",
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

    add_effective_value(
        "async_scheduling",
        [
            (
                full_sched_cfg.get("async_scheduling"),
                "effective_engine_config.scheduler_config.async_scheduling",
            ),
            (engine_args.get("async_scheduling"), "engine_args_from_input.async_scheduling"),
            (parsed_cli.get("async_scheduling"), "parsed_cli_from_input.async_scheduling"),
        ],
    )
    add_effective_value(
        "scheduler_policy",
        [
            (full_sched_cfg.get("policy"), "effective_engine_config.scheduler_config.policy"),
            (engine_args.get("scheduling_policy"), "engine_args_from_input.scheduling_policy"),
            (parsed_cli.get("scheduling_policy"), "parsed_cli_from_input.scheduling_policy"),
        ],
    )
    for field_name in [
        "max_num_partial_prefills",
        "max_long_partial_prefills",
        "long_prefill_token_threshold",
        "max_num_scheduled_tokens",
        "max_num_encoder_input_tokens",
        "scheduler_reserve_full_isl",
        "disable_chunked_mm_input",
        "disable_hybrid_kv_cache_manager",
    ]:
        add_effective_value(
            field_name,
            [
                (
                    full_sched_cfg.get(field_name),
                    f"effective_engine_config.scheduler_config.{field_name}",
                ),
                (engine_args.get(field_name), f"engine_args_from_input.{field_name}"),
                (parsed_cli.get(field_name), f"parsed_cli_from_input.{field_name}"),
            ],
        )

    for field_name in [
        "block_size",
        "kv_cache_memory_bytes",
        "kv_offloading_backend",
        "kv_offloading_size",
        "kv_sharing_fast_prefill",
        "sliding_window",
        "prefix_caching_hash_algo",
        "calculate_kv_scales",
    ]:
        add_effective_value(
            field_name,
            [
                (
                    full_cache_cfg.get(field_name),
                    f"effective_engine_config.cache_config.{field_name}",
                ),
                (engine_args.get(field_name), f"engine_args_from_input.{field_name}"),
                (parsed_cli.get(field_name), f"parsed_cli_from_input.{field_name}"),
            ],
        )

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
        ]
    )
    values["attention_backend"] = attention_backend
    sources["attention_backend"] = source
    if resolved_attention_backend:
        values["attention_backend_details"] = resolved_attention_backend

    values["flashinfer_present"] = _optional_import("flashinfer") is not None
    sources["flashinfer_present"] = "runtime_import.flashinfer"

    values["_usage_context"] = usage_context
    if fallback_mode:
        values["_effective_mode"] = "fallback"
    elif is_cfg_dict:
        values["_effective_mode"] = "full_vllm_config"
    else:
        values["_effective_mode"] = "unavailable"
    values["_sources"] = sources
    return values


def _build_metadata(args: argparse.Namespace) -> dict[str, Any]:
    vllm_mod = _optional_import("vllm")
    torch_mod = _optional_import("torch")
    version = getattr(vllm_mod, "__version__", None) if vllm_mod else None
    module_path = getattr(vllm_mod, "__file__", None) if vllm_mod else None
    torch_version = getattr(torch_mod, "__version__", None) if torch_mod else None

    return {
        "python": sys.version,
        "executable": sys.executable,
        "vllm_version": version,
        "vllm_module_path": module_path,
        "torch_version": torch_version,
        "argv": sys.argv,
        "options": {
            "pid": args.pid,
            "redact_pid_env": args.redact_pid_env,
            "model_path_override": getattr(args, "model_path_override", ""),
            "fallback_command_line": getattr(args, "fallback_command_line", ""),
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
        "/proc/<pid>/environ, applies /proc/<pid>/cwd, infers vLLM CLI args, "
        "and uses them as input.",
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
    parser.add_argument(
        "--model-path-override",
        type=str,
        default="",
        help=(
            "Use this already-resolved local model path for vLLM config "
            "parsing instead of the model id from /proc/<pid>/cmdline."
        ),
    )
    parser.add_argument(
        "--fallback-command-line",
        type=str,
        default="",
        help=(
            "Original vLLM serve command line to use if the target PID is a "
            "worker process whose /proc/<pid>/cmdline no longer contains the "
            "serve arguments."
        ),
    )
    parser.add_argument(
        "--effective-timeout-seconds",
        type=float,
        default=45,
        help=(
            "Timeout for model-aware effective config extraction. "
            "Use <=0 to disable."
        ),
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    out_path: Path | None = None
    if args.out != "-":
        out_path = Path(args.out)
        if not out_path.is_absolute():
            out_path = Path.cwd() / out_path

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
    except Exception as exc:
        errors["pid.read"] = repr(exc)
    else:
        pid_cwd: str | None = None
        pid_cwd_applied: str | None = None
        pid_cwd_error: str | None = None

        try:
            pid_cwd = _read_pid_cwd(args.pid)
        except Exception as exc:
            pid_cwd_error = repr(exc)

        requested_pid = args.pid
        requested_cmdline = list(pid_cmdline)
        requested_cwd = pid_cwd
        resolution = _resolve_vllm_cli_process(
            requested_pid=requested_pid,
            requested_cmdline=requested_cmdline,
            requested_cwd=requested_cwd,
            fallback_command_line=getattr(args, "fallback_command_line", ""),
        )
        inferred_args = resolution.inferred_args
        infer_warning = resolution.warning

        if resolution.pid != requested_pid:
            try:
                pid_cmdline = _read_pid_cmdline(resolution.pid)
                pid_environ = _read_pid_environ(resolution.pid)
            except Exception as exc:
                errors["pid.resolved_read"] = repr(exc)
                pid_cmdline = resolution.cmdline
            else:
                pid_cwd = None
                pid_cwd_error = None
                try:
                    pid_cwd = _read_pid_cwd(resolution.pid)
                except Exception as exc:
                    pid_cwd_error = repr(exc)
        else:
            pid_cmdline = resolution.cmdline

        inferred_usage_context = _infer_usage_context_from_cmdline(pid_cmdline)
        original_inferred_args = list(inferred_args)
        model_path_override = getattr(args, "model_path_override", "").strip()
        model_path_override_source = "argument" if model_path_override else ""
        if inferred_args and not model_path_override:
            auto_override, auto_source = _auto_model_path_override_from_cache(
                inferred_args,
                pid_environ,
            )
            if auto_override:
                model_path_override = auto_override
                model_path_override_source = auto_source or "hf_cache"
        model_path_override_applied = False
        if inferred_args and model_path_override:
            inferred_args, model_path_override_applied = _override_vllm_model_arg(
                inferred_args,
                model_path_override,
            )

        applied_env_keys = _apply_pid_environment(pid_environ)
        if pid_cwd is not None:
            try:
                pid_cwd_applied = _apply_pid_cwd(pid_cwd)
            except Exception as exc:
                pid_cwd_error = repr(exc)
        if pid_cwd_error is not None:
            warnings["pid.cwd"] = pid_cwd_error

        _force_platform_if_needed(pid_environ, errors)

        pid_process = {
            "pid": resolution.pid,
            "cmdline": pid_cmdline,
            "inferred_vllm_cli_args": inferred_args,
            "inferred_usage_context": inferred_usage_context,
            "cli_inference_source": resolution.source,
            "model_path_override": model_path_override,
            "model_path_override_source": model_path_override_source,
            "model_path_override_applied": model_path_override_applied,
            "cwd": pid_cwd,
            "cwd_applied": pid_cwd_applied,
            "environ": _redact_env(pid_environ)
            if args.redact_pid_env
            else pid_environ,
            "pid_env_applied": "allowlisted",
            "pid_env_applied_keys": applied_env_keys,
        }
        if requested_pid != resolution.pid:
            pid_process["requested_pid"] = requested_pid
            pid_process["requested_cmdline"] = requested_cmdline
            pid_process["requested_cwd"] = requested_cwd
        fallback_command_line = getattr(args, "fallback_command_line", "").strip()
        if fallback_command_line:
            pid_process["fallback_command_line"] = fallback_command_line
        if resolution.fallback_cmdline is not None:
            pid_process["fallback_cmdline"] = resolution.fallback_cmdline
        if model_path_override_applied:
            pid_process["original_inferred_vllm_cli_args"] = original_inferred_args
        if pid_cwd_error is not None:
            pid_process["cwd_error"] = pid_cwd_error
        if not inferred_args:
            errors["pid.inferred_cli_args"] = (
                infer_warning
                or "Could not infer vLLM CLI args from /proc/<pid>/cmdline."
            )

    if pid_process and pid_process["inferred_vllm_cli_args"]:
        inferred_args = pid_process["inferred_vllm_cli_args"]
        parsed_cli_from_input, engine_args_from_input = _parse_vllm_cli_input(
            inferred_args,
            errors,
            parse_timeout_seconds=getattr(args, "effective_timeout_seconds", 45),
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
        effective_timeout_seconds=getattr(args, "effective_timeout_seconds", 45),
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
        assert out_path is not None
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(payload, encoding="utf-8")

    if args.fail_on_error and errors:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
