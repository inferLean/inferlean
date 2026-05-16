from __future__ import annotations

import argparse
import contextlib
import dataclasses
import enum
import importlib.util
import io
import json
import os
import sys
import tempfile
import time
import types
import unittest
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).resolve().parents[1] / "dump_vllm_defaults.py"
SPEC = importlib.util.spec_from_file_location("dump_vllm_defaults", MODULE_PATH)
dump_vllm_defaults = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(dump_vllm_defaults)


MISSING = object()


def mod(name: str, **attrs: object) -> types.ModuleType:
    module = types.ModuleType(name)
    for key, value in attrs.items():
        setattr(module, key, value)
    return module


@contextlib.contextmanager
def patched_modules(mapping: dict[str, types.ModuleType]):
    originals = {name: sys.modules.get(name, MISSING) for name in mapping}
    try:
        sys.modules.update(mapping)
        for name, module in mapping.items():
            if "." not in name:
                continue
            parent_name, attr = name.rsplit(".", 1)
            parent = sys.modules.get(parent_name)
            if parent is not None:
                setattr(parent, attr, module)
        yield
    finally:
        for name, original in originals.items():
            if original is MISSING:
                sys.modules.pop(name, None)
            else:
                sys.modules[name] = original


def optional_import_from(mapping: dict[str, object]):
    def fake_import(name: str) -> object | None:
        value = mapping.get(name, MISSING)
        if value is MISSING:
            return None
        if isinstance(value, BaseException):
            raise value
        return value

    return fake_import


class DumpVllmDefaultsTests(unittest.TestCase):
    def test_to_jsonable_covers_supported_shapes_and_edges(self) -> None:
        class Flavor(enum.Enum):
            VANILLA = "vanilla"

        @dataclasses.dataclass
        class Child:
            value: int

        @dataclasses.dataclass
        class Parent:
            path: Path
            child: Child

        class ModelDump:
            def model_dump(self) -> dict[str, object]:
                return {"ok": True}

        class BrokenModelDump:
            def model_dump(self) -> dict[str, object]:
                raise RuntimeError("boom")

            def __repr__(self) -> str:
                return "broken-model"

        class StructLike:
            __struct_fields__ = ("name", "count")

            def __init__(self) -> None:
                self.name = "s"
                self.count = 3

        recursive: list[object] = []
        recursive.append(recursive)
        too_deep = [[[[[[[[[["leaf"]]]]]]]]]]

        cases = [
            (None, None),
            (True, True),
            (42, 42),
            (3.5, 3.5),
            ("x", "x"),
            (Flavor.VANILLA, {"enum": f"{__name__}.DumpVllmDefaultsTests.test_to_jsonable_covers_supported_shapes_and_edges.<locals>.Flavor", "name": "VANILLA", "value": "vanilla"}),
            (Path("/tmp/model"), "/tmp/model"),
            (argparse.Namespace(alpha=1), {"alpha": 1}),
            (Parent(Path("/tmp/p"), Child(2)), {"path": "/tmp/p", "child": {"value": 2}}),
            (ModelDump(), {"ok": True}),
            ({1: "one", "two": (2, 3)}, {"1": "one", "two": [2, 3]}),
            (StructLike(), {"name": "s", "count": 3}),
            (BrokenModelDump(), "broken-model"),
            (recursive, [dump_vllm_defaults.RECURSIVE]),
        ]
        for value, expected in cases:
            with self.subTest(value=type(value).__name__):
                self.assertEqual(dump_vllm_defaults._to_jsonable(value), expected)

        self.assertEqual(
            sorted(dump_vllm_defaults._to_jsonable({"b", "a"})),
            ["a", "b"],
        )
        self.assertEqual(
            dump_vllm_defaults._to_jsonable(too_deep),
            [[[[[[[[["<max_depth:list>"]]]]]]]]],
        )
        self.assertTrue(
            dump_vllm_defaults._to_jsonable(
                DumpVllmDefaultsTests.test_to_jsonable_covers_supported_shapes_and_edges
            ).endswith(".DumpVllmDefaultsTests.test_to_jsonable_covers_supported_shapes_and_edges")
        )

    def test_field_default_resolution_covers_dataclass_and_pydantic_variants(self) -> None:
        class FakeUndefined:
            pass

        pydantic_undefined = FakeUndefined()

        class FakeFieldInfo:
            def __init__(
                self,
                default: object = dataclasses.MISSING,
                default_factory: object | None = None,
            ) -> None:
                self.default = default
                self.default_factory = default_factory

        def raising_factory() -> object:
            raise RuntimeError("factory boom")

        undefined_marker = pydantic_undefined

        @dataclasses.dataclass
        class Defaults:
            required: str
            literal: int = 7
            factory: list[str] = dataclasses.field(default_factory=lambda: ["x"])
            factory_error: object = dataclasses.field(default_factory=raising_factory)
            pydantic_default: object = FakeFieldInfo(default="pd")
            pydantic_factory: object = FakeFieldInfo(default_factory=lambda: "pf")
            pydantic_missing: object = FakeFieldInfo()
            pydantic_undefined: object = FakeFieldInfo(default=undefined_marker)

        fields = {field.name: field for field in dataclasses.fields(Defaults)}
        fake_import = optional_import_from(
            {
                "pydantic.fields": mod("pydantic.fields", FieldInfo=FakeFieldInfo),
                "pydantic_core": mod(
                    "pydantic_core",
                    PydanticUndefined=pydantic_undefined,
                ),
            }
        )
        with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_import):
            self.assertEqual(
                dump_vllm_defaults._extract_dataclass_defaults(Defaults),
                {
                    "required": dump_vllm_defaults.NO_DEFAULT,
                    "literal": 7,
                    "factory": ["x"],
                    "factory_error": "<default_factory_error:factory boom>",
                    "pydantic_default": "pd",
                    "pydantic_factory": "pf",
                    "pydantic_missing": dump_vllm_defaults.NO_DEFAULT,
                    "pydantic_undefined": dump_vllm_defaults.NO_DEFAULT,
                },
            )
            self.assertEqual(
                dump_vllm_defaults._resolve_field_default(fields["required"]),
                (False, dump_vllm_defaults.NO_DEFAULT),
            )

    def test_cli_config_and_engine_default_extractors_with_fake_vllm_modules(self) -> None:
        class FlexibleArgumentParser(argparse.ArgumentParser):
            pass

        def make_arg_parser(parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
            parser.add_argument("--port", type=int, default=8000)
            return parser

        @dataclasses.dataclass
        class EngineArgs:
            gpu_memory_utilization: float = 0.9

        @dataclasses.dataclass
        class AsyncEngineArgs:
            model: str = "default-model"

            @classmethod
            def add_cli_args(cls, parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
                parser.add_argument("--tensor-parallel-size", type=int, default=1)
                return parser

        @dataclasses.dataclass
        class GoodConfig:
            block_size: int = 16

        GoodConfig.__module__ = "vllm.config"

        @dataclasses.dataclass
        class FallbackConfig:
            cache_dtype: str = "auto"

        FallbackConfig.__module__ = "vllm.config.submodule"

        @dataclasses.dataclass
        class ExternalConfig:
            ignored: bool = True

        ExternalConfig.__module__ = "elsewhere"

        class BrokenConfig:
            __module__ = "vllm.config"

        def get_kwargs(cls: type[object]) -> dict[str, dict[str, object]]:
            if cls is GoodConfig:
                return {"block_size": {"default": 32}, "required": {}}
            raise RuntimeError("fallback please")

        argparse_utils = mod(
            "vllm.utils.argparse_utils",
            FlexibleArgumentParser=FlexibleArgumentParser,
        )
        cli_args = mod("vllm.entrypoints.openai.cli_args", make_arg_parser=make_arg_parser)
        arg_utils = mod(
            "vllm.engine.arg_utils",
            EngineArgs=EngineArgs,
            AsyncEngineArgs=AsyncEngineArgs,
            get_kwargs=get_kwargs,
        )
        config = mod(
            "vllm.config",
            GoodConfig=GoodConfig,
            FallbackConfig=FallbackConfig,
            ExternalConfig=ExternalConfig,
            BrokenConfig=BrokenConfig,
            OtherThing=GoodConfig,
            ValueConfig=object(),
        )

        fake_import = optional_import_from(
            {
                "vllm.utils.argparse_utils": argparse_utils,
                "vllm.entrypoints.openai.cli_args": cli_args,
                "vllm.engine.arg_utils": arg_utils,
                "vllm.config": config,
            }
        )
        errors: dict[str, str] = {}
        with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_import):
            self.assertEqual(
                dump_vllm_defaults._extract_cli_defaults(errors),
                {
                    "serve_make_arg_parser": {"port": 8000},
                    "async_engine_add_cli_args": {"tensor_parallel_size": 1},
                },
            )
            self.assertEqual(
                dump_vllm_defaults._extract_config_defaults(errors),
                {
                    "FallbackConfig": {"cache_dtype": "auto"},
                    "GoodConfig": {
                        "block_size": 32,
                        "required": dump_vllm_defaults.NO_DEFAULT,
                    },
                },
            )
            self.assertEqual(
                dump_vllm_defaults._extract_engine_args_defaults(errors),
                {
                    "EngineArgs": {"gpu_memory_utilization": 0.9},
                    "AsyncEngineArgs": {"model": "default-model"},
                },
            )
        self.assertEqual(errors, {})

        missing_errors: dict[str, str] = {}
        with mock.patch.object(
            dump_vllm_defaults,
            "_optional_import",
            optional_import_from({}),
        ):
            self.assertEqual(dump_vllm_defaults._extract_config_defaults(missing_errors), {})
            self.assertEqual(
                dump_vllm_defaults._extract_engine_args_defaults(missing_errors),
                {},
            )
        self.assertEqual(
            missing_errors,
            {
                "config.module": "Could not import vllm.config",
                "engine_args.module": "Could not import vllm.engine.arg_utils",
            },
        )

    def test_cli_default_extraction_records_regular_exceptions(self) -> None:
        class BrokenAsyncEngineArgs:
            @classmethod
            def add_cli_args(cls, parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
                raise RuntimeError("async broken")

        def broken_make_arg_parser(parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
            raise RuntimeError("serve broken")

        fake_import = optional_import_from(
            {
                "vllm.entrypoints.openai.cli_args": mod(
                    "vllm.entrypoints.openai.cli_args",
                    make_arg_parser=broken_make_arg_parser,
                ),
                "vllm.engine.arg_utils": mod(
                    "vllm.engine.arg_utils",
                    AsyncEngineArgs=BrokenAsyncEngineArgs,
                ),
            }
        )
        errors: dict[str, str] = {}
        with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_import):
            self.assertEqual(dump_vllm_defaults._extract_cli_defaults(errors), {})

        self.assertIn("RuntimeError('serve broken')", errors["cli.serve_make_arg_parser"])
        self.assertIn("RuntimeError('async broken')", errors["cli.async_engine_add_cli_args"])

    def test_cli_default_extraction_records_argparse_exits(self) -> None:
        class ExitingAsyncEngineArgs:
            @classmethod
            def add_cli_args(cls, parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
                parser.add_argument("required_async")
                return parser

        def make_required_parser(parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
            parser.add_argument("required_model")
            return parser

        fake_import = optional_import_from(
            {
                "vllm.entrypoints.openai.cli_args": mod(
                    "vllm.entrypoints.openai.cli_args",
                    make_arg_parser=make_required_parser,
                ),
                "vllm.engine.arg_utils": mod(
                    "vllm.engine.arg_utils",
                    AsyncEngineArgs=ExitingAsyncEngineArgs,
                ),
            }
        )
        errors: dict[str, str] = {}
        with (
            mock.patch.object(dump_vllm_defaults, "_optional_import", fake_import),
            contextlib.redirect_stderr(io.StringIO()),
        ):
            self.assertEqual(dump_vllm_defaults._extract_cli_defaults(errors), {})
        self.assertIn("parse_args exited", errors["cli.serve_make_arg_parser"])
        self.assertIn("parse_args exited", errors["cli.async_engine_add_cli_args"])

    def test_import_proc_wrappers_and_parser_default_edges(self) -> None:
        self.assertIs(dump_vllm_defaults._optional_import("math"), sys.modules["math"])
        self.assertIsNone(dump_vllm_defaults._optional_import("definitely_missing_module"))

        parser = argparse.ArgumentParser()
        parser.add_argument("--flag", action="store_true")
        parser.add_argument("--name", default="default-name")
        self.assertEqual(
            dump_vllm_defaults._extract_parser_defaults(parser),
            {"flag": False, "name": "default-name"},
        )

        with mock.patch.object(
            dump_vllm_defaults,
            "_read_proc_null_separated",
            side_effect=[["vllm", "m"], ["A=1", "NO_EQUALS", "B=two=three"]],
        ) as read_nulls:
            self.assertEqual(dump_vllm_defaults._read_pid_cmdline(123), ["vllm", "m"])
            self.assertEqual(
                dump_vllm_defaults._read_pid_environ(456),
                {"A": "1", "B": "two=three"},
            )
        self.assertEqual(
            [call.args[0] for call in read_nulls.call_args_list],
            [Path("/proc/123/cmdline"), Path("/proc/456/environ")],
        )
        with mock.patch.object(os, "readlink", return_value="/srv/vllm") as readlink:
            self.assertEqual(dump_vllm_defaults._read_pid_cwd(789), "/srv/vllm")
        readlink.assert_called_once_with("/proc/789/cwd")

    def test_proc_and_commandline_helpers_cover_supported_and_unsupported_shapes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            path = Path(tmp_dir) / "nulls"
            path.write_bytes(b"KEY=value\x00NO_EQUALS\x00A=B=C\x00\x00")
            self.assertEqual(
                dump_vllm_defaults._read_proc_null_separated(path),
                ["KEY=value", "NO_EQUALS", "A=B=C"],
            )

        self.assertEqual(
            dump_vllm_defaults._strip_optional_serve(["serve", "model", "--port", "1"]),
            ["model", "--port", "1"],
        )
        self.assertEqual(
            dump_vllm_defaults._strip_optional_serve(["model", "serve"]),
            ["model", "serve"],
        )

        command_cases = [
            ([], ([], "empty cmdline")),
            (["vllm", "serve", "facebook/opt"], (["facebook/opt"], None)),
            (["/usr/bin/vllm", "--port", "8000"], (["--port", "8000"], None)),
            (["python", "-m", "vllm", "serve", "m"], (["m"], None)),
            (["python3.12", "-m", "vllm.entrypoints.openai.api_server", "m"], (["m"], None)),
            (["python", "/venv/bin/vllm", "serve", "m"], (["m"], None)),
            (["uv", "run", "vllm", "serve", "m"], (["m"], None)),
            (["uv", "run", "python", "-m", "vllm", "m"], (["m"], None)),
            (["uv", "run", "python", "/venv/bin/vllm", "m"], (["m"], None)),
            (
                ["uv", "run", "--with", "vllm", "vllm", "m"],
                ([], "unsupported vLLM launch shape: uv"),
            ),
            (
                ["python", "-m", "not_vllm", "m"],
                ([], "unsupported vLLM launch shape: python"),
            ),
            (
                ["bash", "-lc", "vllm serve m"],
                ([], "unsupported vLLM launch shape: bash"),
            ),
        ]
        for cmdline, expected in command_cases:
            with self.subTest(cmdline=cmdline):
                self.assertEqual(
                    dump_vllm_defaults._infer_vllm_cli_args_from_cmdline(cmdline),
                    expected,
                )
        override = "/home/bale1/.cache/huggingface/hub/models--google--gemma/snapshots/abc"
        override_cases = [
            (["google/gemma", "--port", "8000"], [override, "--port", "8000"]),
            (["--model", "google/gemma", "--port", "8000"], ["--model", override, "--port", "8000"]),
            (["--model=google/gemma"], ["--model=" + override]),
            (["--model-tag", "google/gemma"], ["--model-tag", override]),
            (["--port", "8000", "google/gemma"], ["--port", "8000", override]),
            (["--async-scheduling", "google/gemma"], ["--async-scheduling", override]),
            (
                ["--limit-mm-per-prompt", '{"video": 0}', "google/gemma"],
                ["--limit-mm-per-prompt", '{"video": 0}', override],
            ),
            (["--port", "8000"], [override, "--port", "8000"]),
            ([], [override]),
        ]
        for raw_cli_args, expected_args in override_cases:
            with self.subTest(raw_cli_args=raw_cli_args):
                got_args, applied = dump_vllm_defaults._override_vllm_model_arg(
                    raw_cli_args,
                    override,
                )
                self.assertTrue(applied)
                self.assertEqual(got_args, expected_args)
        got_args, applied = dump_vllm_defaults._override_vllm_model_arg(
            ["google/gemma"],
            "",
        )
        self.assertFalse(applied)
        self.assertEqual(got_args, ["google/gemma"])

    def test_pid_environment_helpers_filter_redact_apply_and_preserve_pythonpath_order(self) -> None:
        env = {
            "CUDA_VISIBLE_DEVICES": "0",
            "HOME": "/home/vllm",
            "HF_HOME": "/cache/hf-home",
            "HF_HUB_CACHE": "/cache/hf-hub",
            "HF_HUB_OFFLINE": "1",
            "HUGGINGFACE_HUB_CACHE": "/cache/huggingface-hub",
            "HUGGING_FACE_HUB_TOKEN": "hf-secret",
            "TRANSFORMERS_CACHE": "/cache/transformers",
            "VLLM_ATTENTION_BACKEND": "FLASH_ATTN",
            "NCCL_DEBUG": "INFO",
            "XDG_CACHE_HOME": "/tmp/cache",
            "VLLM_API_KEY": "secret",
            "AWS_SECRET_ACCESS_KEY": "secret",
            "UNRELATED": "ignored",
            "PYTHONPATH": os.pathsep.join(["/tmp/first", "", "/tmp/second"]),
        }
        self.assertTrue(dump_vllm_defaults._is_sensitive_env_key("hf_token"))
        self.assertTrue(dump_vllm_defaults._is_sensitive_env_key("PRIVATE_KEY_PATH"))
        self.assertFalse(dump_vllm_defaults._is_sensitive_env_key("VLLM_TARGET_DEVICE"))
        self.assertEqual(
            dump_vllm_defaults._redact_env(env)["AWS_SECRET_ACCESS_KEY"],
            "<redacted>",
        )
        self.assertEqual(
            dump_vllm_defaults._filter_pid_environment(env),
            {
                "CUDA_VISIBLE_DEVICES": "0",
                "HOME": "/home/vllm",
                "HF_HOME": "/cache/hf-home",
                "HF_HUB_CACHE": "/cache/hf-hub",
                "HF_HUB_OFFLINE": "1",
                "HUGGINGFACE_HUB_CACHE": "/cache/huggingface-hub",
                "HUGGING_FACE_HUB_TOKEN": "hf-secret",
                "TRANSFORMERS_CACHE": "/cache/transformers",
                "VLLM_ATTENTION_BACKEND": "FLASH_ATTN",
                "NCCL_DEBUG": "INFO",
                "XDG_CACHE_HOME": "/tmp/cache",
                "VLLM_API_KEY": "secret",
                "PYTHONPATH": os.pathsep.join(["/tmp/first", "", "/tmp/second"]),
            },
        )

        original_environ = os.environ.copy()
        original_path = list(sys.path)
        try:
            applied = dump_vllm_defaults._apply_pid_environment(env)
            self.assertEqual(
                applied,
                [
                    "CUDA_VISIBLE_DEVICES",
                    "HF_HOME",
                    "HF_HUB_CACHE",
                    "HF_HUB_OFFLINE",
                    "HOME",
                    "HUGGINGFACE_HUB_CACHE",
                    "HUGGING_FACE_HUB_TOKEN",
                    "NCCL_DEBUG",
                    "PYTHONPATH",
                    "TRANSFORMERS_CACHE",
                    "VLLM_API_KEY",
                    "VLLM_ATTENTION_BACKEND",
                    "XDG_CACHE_HOME",
                ],
            )
            self.assertEqual(os.environ["VLLM_ATTENTION_BACKEND"], "FLASH_ATTN")
            self.assertLess(sys.path.index("/tmp/first"), sys.path.index("/tmp/second"))
            dump_vllm_defaults._apply_pythonpath("/tmp/first")
            self.assertEqual(sys.path.count("/tmp/first"), 1)
        finally:
            os.environ.clear()
            os.environ.update(original_environ)
            sys.path[:] = original_path

        original_cwd = os.getcwd()
        try:
            with tempfile.TemporaryDirectory() as tmp_dir:
                self.assertEqual(
                    Path(dump_vllm_defaults._apply_pid_cwd(tmp_dir)),
                    Path(tmp_dir).resolve(),
                )
        finally:
            os.chdir(original_cwd)

    def test_usage_context_and_platform_resolution(self) -> None:
        class UsageContext(enum.Enum):
            OPENAI_API_SERVER = "openai"
            LLM_CLASS = "llm"

        fake_usage_import = optional_import_from(
            {"vllm.usage.usage_lib": mod("vllm.usage.usage_lib", UsageContext=UsageContext)}
        )
        with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_usage_import):
            self.assertIs(
                dump_vllm_defaults._resolve_usage_context("OPENAI_API_SERVER"),
                UsageContext.OPENAI_API_SERVER,
            )
            self.assertIs(
                dump_vllm_defaults._resolve_usage_context("llm"),
                UsageContext.LLM_CLASS,
            )
            self.assertIsNone(dump_vllm_defaults._resolve_usage_context("missing"))
        with mock.patch.object(
            dump_vllm_defaults,
            "_optional_import",
            optional_import_from({}),
        ):
            self.assertIsNone(dump_vllm_defaults._resolve_usage_context("OPENAI_API_SERVER"))

        class EmptyPlatform:
            device_type = ""

        class ExistingPlatform:
            device_type = "cuda"

        class CudaPlatform:
            device_type = "cuda"

        class CpuPlatform:
            device_type = "cpu"

        class XPUPlatform:
            device_type = "xpu"

        platforms = mod("vllm.platforms", current_platform=EmptyPlatform())
        modules = {
            "vllm": mod("vllm", platforms=platforms),
            "vllm.platforms": platforms,
            "vllm.platforms.cuda": mod("vllm.platforms.cuda", CudaPlatform=CudaPlatform),
            "vllm.platforms.cpu": mod("vllm.platforms.cpu", CpuPlatform=CpuPlatform),
            "vllm.platforms.xpu": mod("vllm.platforms.xpu", XPUPlatform=XPUPlatform),
        }
        with patched_modules(modules):
            errors: dict[str, str] = {}
            platforms.current_platform = EmptyPlatform()
            dump_vllm_defaults._force_platform_if_needed({}, errors)
            self.assertIsInstance(platforms.current_platform, EmptyPlatform)
            for target, expected_type in [
                ("cuda", CudaPlatform),
                ("cpu", CpuPlatform),
                ("xpu", XPUPlatform),
            ]:
                with self.subTest(target=target):
                    platforms.current_platform = EmptyPlatform()
                    dump_vllm_defaults._force_platform_if_needed(
                        {"VLLM_TARGET_DEVICE": target},
                        errors,
                    )
                    self.assertIsInstance(platforms.current_platform, expected_type)
            platforms.current_platform = ExistingPlatform()
            dump_vllm_defaults._force_platform_if_needed(
                {"VLLM_TARGET_DEVICE": "cpu"},
                errors,
            )
            self.assertIsInstance(platforms.current_platform, ExistingPlatform)
        self.assertEqual(errors, {})

    def test_parse_vllm_cli_input_covers_missing_success_and_regular_exception(self) -> None:
        @dataclasses.dataclass
        class AsyncEngineArgs:
            model: str

            @classmethod
            def from_cli_args(cls, parsed_ns: argparse.Namespace) -> "AsyncEngineArgs":
                return cls(model=parsed_ns.model)

        def make_arg_parser(parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
            parser.add_argument("model")
            parser.add_argument("--port", type=int, default=8000)
            return parser

        fake_import = optional_import_from(
            {
                "vllm.entrypoints.openai.cli_args": mod(
                    "vllm.entrypoints.openai.cli_args",
                    make_arg_parser=make_arg_parser,
                ),
                "vllm.engine.arg_utils": mod(
                    "vllm.engine.arg_utils",
                    AsyncEngineArgs=AsyncEngineArgs,
                ),
            }
        )
        errors: dict[str, str] = {}
        with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_import):
            parsed, engine_args = dump_vllm_defaults._parse_vllm_cli_input(
                ["facebook/opt", "--port", "9000"],
                errors,
            )
        self.assertEqual(parsed, {"model": "facebook/opt", "port": 9000})
        self.assertEqual(dataclasses.asdict(engine_args), {"model": "facebook/opt"})
        self.assertEqual(errors, {})

        missing_errors: dict[str, str] = {}
        with mock.patch.object(
            dump_vllm_defaults,
            "_optional_import",
            optional_import_from({}),
        ):
            self.assertEqual(
                dump_vllm_defaults._parse_vllm_cli_input(["m"], missing_errors),
                (None, None),
            )
        self.assertEqual(
            missing_errors,
            {"input_cli_args": "Required vLLM modules unavailable."},
        )

        class BrokenAsyncEngineArgs:
            @classmethod
            def from_cli_args(cls, parsed_ns: argparse.Namespace) -> object:
                raise RuntimeError("from args failed")

        fake_broken_import = optional_import_from(
            {
                "vllm.entrypoints.openai.cli_args": mod(
                    "vllm.entrypoints.openai.cli_args",
                    make_arg_parser=make_arg_parser,
                ),
                "vllm.engine.arg_utils": mod(
                    "vllm.engine.arg_utils",
                    AsyncEngineArgs=BrokenAsyncEngineArgs,
                ),
            }
        )
        broken_errors: dict[str, str] = {}
        with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_broken_import):
            self.assertEqual(
                dump_vllm_defaults._parse_vllm_cli_input(["m"], broken_errors),
                (None, None),
            )
        self.assertIn("RuntimeError('from args failed')", broken_errors["input_cli_args_parse"])

        class SlowAsyncEngineArgs:
            @classmethod
            def from_cli_args(cls, parsed_ns: argparse.Namespace) -> object:
                _ = parsed_ns
                time.sleep(1)
                return object()

        fake_slow_import = optional_import_from(
            {
                "vllm.entrypoints.openai.cli_args": mod(
                    "vllm.entrypoints.openai.cli_args",
                    make_arg_parser=make_arg_parser,
                ),
                "vllm.engine.arg_utils": mod(
                    "vllm.engine.arg_utils",
                    AsyncEngineArgs=SlowAsyncEngineArgs,
                ),
            }
        )
        slow_errors: dict[str, str] = {}
        with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_slow_import):
            self.assertEqual(
                dump_vllm_defaults._parse_vllm_cli_input(
                    ["m"],
                    slow_errors,
                    parse_timeout_seconds=0.01,
                ),
                (None, None),
            )
        self.assertIn("input CLI parsing timed out", slow_errors["input_cli_args_parse"])

    def test_parse_vllm_cli_input_records_argparse_system_exit_as_parse_error(self) -> None:
        def make_arg_parser(parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
            parser.add_argument("model")
            return parser

        fake_import = optional_import_from(
            {
                "vllm.entrypoints.openai.cli_args": mod(
                    "vllm.entrypoints.openai.cli_args",
                    make_arg_parser=make_arg_parser,
                ),
                "vllm.engine.arg_utils": mod(
                    "vllm.engine.arg_utils",
                    AsyncEngineArgs=object,
                ),
            }
        )
        errors: dict[str, str] = {}
        with (
            mock.patch.object(dump_vllm_defaults, "_optional_import", fake_import),
            contextlib.redirect_stderr(io.StringIO()),
        ):
            self.assertEqual(
                dump_vllm_defaults._parse_vllm_cli_input([], errors),
                (None, None),
            )
        self.assertIn("input_cli_args_parse", errors)

    def test_runtime_attention_backend_serialization_and_error_paths(self) -> None:
        class ModelConfig:
            dtype = "torch.float16"

            def get_head_size(self) -> int:
                return 128

        class CacheConfig:
            cache_dtype = "auto"

        @dataclasses.dataclass
        class VllmConfig:
            model_config: object
            cache_config: object

        class Backend:
            @classmethod
            def get_name(cls) -> str:
                return "FLASH_ATTN"

        class BackendByClassName:
            @classmethod
            def get_name(cls) -> None:
                return None

        class BlankBackendName:
            @classmethod
            def get_name(cls) -> str:
                return " "

        @contextlib.contextmanager
        def set_current_vllm_config(config: object):
            yield

        def get_attn_backend(**kwargs: object) -> type[Backend]:
            self.assertEqual(
                kwargs,
                {"head_size": 128, "dtype": "torch.float16", "kv_cache_dtype": "auto"},
            )
            return Backend

        modules = {
            "vllm": mod("vllm"),
            "vllm.config": mod("vllm.config"),
            "vllm.config.vllm": mod(
                "vllm.config.vllm",
                set_current_vllm_config=set_current_vllm_config,
            ),
            "vllm.v1": mod("vllm.v1"),
            "vllm.v1.attention": mod("vllm.v1.attention"),
            "vllm.v1.attention.selector": mod(
                "vllm.v1.attention.selector",
                get_attn_backend=get_attn_backend,
            ),
        }
        cfg = VllmConfig(model_config=ModelConfig(), cache_config=CacheConfig())
        with patched_modules(modules):
            errors: dict[str, str] = {}
            self.assertEqual(
                dump_vllm_defaults._resolve_runtime_attention_backend(cfg, errors),
                {
                    "name": "FLASH_ATTN",
                    "class": f"{__name__}.DumpVllmDefaultsTests.test_runtime_attention_backend_serialization_and_error_paths.<locals>.Backend",
                    "module": __name__,
                    "source": "vllm.v1.attention.selector.get_attn_backend",
                },
            )
            serialized = dump_vllm_defaults._serialize_effective_engine_config(cfg, errors)
        self.assertEqual(serialized["resolved_attention_backend"]["name"], "FLASH_ATTN")
        self.assertEqual(errors, {})

        modules["vllm.v1.attention.selector"] = mod(
            "vllm.v1.attention.selector",
            get_attn_backend=lambda **kwargs: BackendByClassName,
        )
        with patched_modules(modules):
            self.assertEqual(
                dump_vllm_defaults._resolve_runtime_attention_backend(cfg, {}),
                {
                    "name": "BackendByClassName",
                    "class": f"{__name__}.DumpVllmDefaultsTests.test_runtime_attention_backend_serialization_and_error_paths.<locals>.BackendByClassName",
                    "module": __name__,
                    "source": "vllm.v1.attention.selector.get_attn_backend",
                },
            )

        modules["vllm.v1.attention.selector"] = mod(
            "vllm.v1.attention.selector",
            get_attn_backend=lambda **kwargs: BlankBackendName,
        )
        with patched_modules(modules):
            self.assertIsNone(
                dump_vllm_defaults._resolve_runtime_attention_backend(cfg, {})
            )

        self.assertIsNone(
            dump_vllm_defaults._resolve_runtime_attention_backend(object(), {})
        )

        def broken_get_attn_backend(**kwargs: object) -> object:
            raise RuntimeError("selector failed")

        modules["vllm.v1.attention.selector"] = mod(
            "vllm.v1.attention.selector",
            get_attn_backend=broken_get_attn_backend,
        )
        with patched_modules(modules):
            errors = {}
            self.assertIsNone(
                dump_vllm_defaults._resolve_runtime_attention_backend(cfg, errors)
            )
        self.assertIn(
            "RuntimeError('selector failed')",
            errors["effective.attention_backend_resolve"],
        )

    def test_effective_engine_config_covers_success_missing_usage_and_fallbacks(self) -> None:
        class UsageContext(enum.Enum):
            OPENAI_API_SERVER = "openai"

        @dataclasses.dataclass
        class EngineArgs:
            model: str = "m"
            failure: str | None = None

            def create_engine_config(self, **kwargs: object) -> object:
                if self.failure == "cuda":
                    self.failure = None
                    raise RuntimeError("No CUDA GPUs are available")
                if self.failure == "generic":
                    raise RuntimeError("plain failure")
                if self.failure == "slow":
                    time.sleep(1)
                    return {"slow": True}
                return {"model_config": {"model": self.model}, "kwargs": kwargs}

        class CpuPlatform:
            device_type = "cpu"

        platforms = mod("vllm.platforms", current_platform=None)
        engine_arg_utils = mod("vllm.engine.arg_utils", current_platform=None)
        modules = {
            "vllm": mod("vllm", platforms=platforms),
            "vllm.usage": mod("vllm.usage"),
            "vllm.usage.usage_lib": mod("vllm.usage.usage_lib", UsageContext=UsageContext),
            "vllm.platforms": platforms,
            "vllm.platforms.cpu": mod("vllm.platforms.cpu", CpuPlatform=CpuPlatform),
            "vllm.engine": mod("vllm.engine", arg_utils=engine_arg_utils),
            "vllm.engine.arg_utils": engine_arg_utils,
        }
        with patched_modules(modules):
            fake_import = optional_import_from({"vllm.usage.usage_lib": modules["vllm.usage.usage_lib"]})
            with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_import):
                errors: dict[str, str] = {}
                warnings: dict[str, str] = {}
                self.assertIsNone(
                    dump_vllm_defaults._extract_effective_engine_config(
                        engine_args_obj=None,
                        usage_context_name="OPENAI_API_SERVER",
                        errors=errors,
                        warnings=warnings,
                    )
                )
                self.assertEqual(
                    errors["effective.engine_args"],
                    "No parsed engine args available from PID CLI arguments.",
                )

                errors = {}
                self.assertEqual(
                    dump_vllm_defaults._extract_effective_engine_config(
                        engine_args_obj=EngineArgs("ok"),
                        usage_context_name="OPENAI_API_SERVER",
                        errors=errors,
                        warnings=warnings,
                    ),
                    {
                        "model_config": {"model": "ok"},
                        "kwargs": {"usage_context": {"enum": f"{__name__}.DumpVllmDefaultsTests.test_effective_engine_config_covers_success_missing_usage_and_fallbacks.<locals>.UsageContext", "name": "OPENAI_API_SERVER", "value": "openai"}, "headless": True},
                    },
                )

                errors = {}
                warnings = {}
                fallback = dump_vllm_defaults._extract_effective_engine_config(
                    engine_args_obj=EngineArgs("cpu", failure="cuda"),
                    usage_context_name="OPENAI_API_SERVER",
                    errors=errors,
                    warnings=warnings,
                )
                self.assertEqual(fallback["_fallback"], "degraded_effective_config")
                self.assertEqual(
                    fallback["_degraded_reason"],
                    "forced CPU fallback because CUDA was unavailable",
                )
                self.assertIn("Fell back to CPU platform", warnings["effective.create_engine_config"])

                errors = {}
                warnings = {}
                generic = dump_vllm_defaults._extract_effective_engine_config(
                    engine_args_obj=EngineArgs("bad", failure="generic"),
                    usage_context_name="OPENAI_API_SERVER",
                    errors=errors,
                    warnings=warnings,
                )
                self.assertEqual(generic["_fallback"], "engine_args_from_input")
                self.assertIn("plain failure", warnings["effective.create_engine_config"])

                errors = {}
                warnings = {}
                timed_out = dump_vllm_defaults._extract_effective_engine_config(
                    engine_args_obj=EngineArgs("slow", failure="slow"),
                    usage_context_name="OPENAI_API_SERVER",
                    errors=errors,
                    warnings=warnings,
                    effective_timeout_seconds=0.01,
                )
                self.assertEqual(timed_out["_fallback"], "engine_args_from_input")
                self.assertIn("timed out", warnings["effective.create_engine_config"])
                self.assertIn("timed out", warnings["effective.derive_model_defaults"])

        fake_missing_usage = optional_import_from({})
        with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_missing_usage):
            errors = {}
            self.assertIsNone(
                dump_vllm_defaults._extract_effective_engine_config(
                    engine_args_obj=EngineArgs("ok"),
                    usage_context_name="OPENAI_API_SERVER",
                    errors=errors,
                    warnings={},
                )
            )
        self.assertIn("Unable to resolve UsageContext", errors["effective.usage_context"])

    def test_cleaners_pick_value_and_effective_parameter_aggregation(self) -> None:
        self.assertEqual(dump_vllm_defaults._pick_value([(None, "a"), (0, "b")]), (0, "b"))
        self.assertEqual(dump_vllm_defaults._pick_value([(None, "a")]), (None, None))
        self.assertIsNone(dump_vllm_defaults._clean_attention_backend(" "))
        self.assertEqual(dump_vllm_defaults._clean_string_value(123), "123")
        self.assertEqual(dump_vllm_defaults._clean_served_model_name(["", "served"]), "served")
        self.assertIsNone(dump_vllm_defaults._clean_served_model_name(["", None]))
        self.assertEqual(dump_vllm_defaults._clean_dtype_value("torch.bfloat16"), "bfloat16")
        self.assertEqual(
            dump_vllm_defaults._requested_model_candidates(
                {"model_tag": "tag", "model": "model"},
                {"model_tag": "engine-tag", "model": "engine-model"},
            ),
            [
                ("tag", "parsed_cli_from_input.model_tag"),
                ("engine-tag", "engine_args_from_input.model_tag"),
                ("model", "parsed_cli_from_input.model"),
                ("engine-model", "engine_args_from_input.model"),
            ],
        )

        parsed_cli = {
            "model_tag": "cli-tag",
            "served_model_name": ["", "cli-served"],
            "tensor_parallel_size": 2,
            "data_parallel_size": 3,
            "pipeline_parallel_size": 4,
            "max_model_len": 1024,
            "max_num_batched_tokens": 2048,
            "max_num_seqs": 64,
            "gpu_memory_utilization": 0.7,
            "kv_cache_dtype": "fp8",
            "enable_prefix_caching": False,
            "enable_chunked_prefill": False,
            "async_scheduling": False,
            "scheduling_policy": "fcfs",
            "max_num_partial_prefills": 1,
            "block_size": 8,
            "quantization": "gptq",
            "dtype": "torch.float16",
            "attention_backend": "FLASH_ATTN_FROM_CLI",
        }
        engine_args = {
            "model": "engine-model",
            "served_model_name": "engine-served",
            "tensor_parallel_size": 8,
            "max_num_batched_tokens": 4096,
            "async_scheduling": True,
            "scheduling_policy": "priority",
            "block_size": 16,
            "quantization": "awq",
            "dtype": "torch.bfloat16",
            "attn_backend": "ENGINE_ATTN",
        }
        effective_cfg = {
            "model_config": {
                "model": "effective-model",
                "served_model_name": "effective-served",
                "max_model_len": 8192,
                "quantization": "fp8",
                "dtype": "torch.float32",
                "attention_backend": "MODEL_ATTN",
            },
            "scheduler_config": {
                "max_num_batched_tokens": 16384,
                "max_num_seqs": 256,
                "enable_chunked_prefill": True,
                "async_scheduling": True,
                "policy": "priority",
                "max_num_partial_prefills": 2,
                "max_long_partial_prefills": 3,
                "long_prefill_token_threshold": 4,
                "max_num_scheduled_tokens": 5,
                "max_num_encoder_input_tokens": 6,
                "scheduler_reserve_full_isl": True,
                "disable_chunked_mm_input": False,
                "disable_hybrid_kv_cache_manager": True,
            },
            "cache_config": {
                "gpu_memory_utilization": 0.95,
                "cache_dtype": "auto",
                "enable_prefix_caching": True,
                "block_size": 32,
                "kv_cache_memory_bytes": 123,
                "kv_offloading_backend": "cpu",
                "kv_offloading_size": 456,
                "kv_sharing_fast_prefill": True,
                "sliding_window": 789,
                "prefix_caching_hash_algo": "sha256",
                "calculate_kv_scales": True,
            },
            "parallel_config": {
                "tensor_parallel_size": 16,
                "data_parallel_size": 1,
                "pipeline_parallel_size": 2,
            },
            "resolved_attention_backend": {
                "name": "RESOLVED_ATTN",
                "class": "Backend",
            },
        }
        with mock.patch.object(
            dump_vllm_defaults,
            "_optional_import",
            optional_import_from({"flashinfer": mod("flashinfer")}),
        ):
            values = dump_vllm_defaults._aggregate_effective_serve_parameters(
                parsed_cli=parsed_cli,
                engine_args=engine_args,
                effective_cfg=effective_cfg,
                usage_context="OPENAI_API_SERVER",
            )
        self.assertEqual(values["model"], "cli-tag")
        self.assertEqual(values["served_model_name"], "engine-served")
        self.assertEqual(values["tensor_parallel_size"], 16)
        self.assertEqual(values["data_parallel_size"], 1)
        self.assertEqual(values["pipeline_parallel_size"], 2)
        self.assertEqual(values["max_model_len"], 8192)
        self.assertEqual(values["max_num_batched_tokens"], 16384)
        self.assertEqual(values["max_num_seqs"], 256)
        self.assertEqual(values["gpu_memory_utilization"], 0.95)
        self.assertEqual(values["kv_cache_dtype"], "auto")
        self.assertEqual(values["enable_prefix_caching"], True)
        self.assertEqual(values["enable_chunked_prefill"], True)
        self.assertEqual(values["scheduler_policy"], "priority")
        for key, expected in {
            "max_num_partial_prefills": 2,
            "max_long_partial_prefills": 3,
            "long_prefill_token_threshold": 4,
            "max_num_scheduled_tokens": 5,
            "max_num_encoder_input_tokens": 6,
            "scheduler_reserve_full_isl": True,
            "disable_chunked_mm_input": False,
            "disable_hybrid_kv_cache_manager": True,
            "block_size": 32,
            "kv_cache_memory_bytes": 123,
            "kv_offloading_backend": "cpu",
            "kv_offloading_size": 456,
            "kv_sharing_fast_prefill": True,
            "sliding_window": 789,
            "prefix_caching_hash_algo": "sha256",
            "calculate_kv_scales": True,
        }.items():
            with self.subTest(aggregate_field=key):
                self.assertEqual(values[key], expected)
        self.assertEqual(values["quantization"], "fp8")
        self.assertEqual(values["dtype"], "float32")
        self.assertEqual(values["attention_backend"], "RESOLVED_ATTN")
        self.assertEqual(values["attention_backend_details"]["class"], "Backend")
        self.assertTrue(values["flashinfer_present"])
        self.assertEqual(values["_effective_mode"], "full_vllm_config")
        self.assertEqual(
            values["_sources"]["attention_backend"],
            "effective_engine_config.resolved_attention_backend.name",
        )

        fallback_cfg = {
            "_fallback": "engine_args_from_input",
            "engine_args": {
                "model": "fallback-model",
                "served_model_name": "fallback-served",
                "tensor_parallel_size": 9,
                "data_parallel_size": 10,
                "pipeline_parallel_size": 11,
                "max_model_len": 12000,
                "max_num_batched_tokens": 13000,
                "max_num_seqs": 14,
                "gpu_memory_utilization": 0.5,
                "kv_cache_dtype": "fp8_e5m2",
                "enable_prefix_caching": False,
                "enable_chunked_prefill": False,
                "quantization": "fallback-quant",
                "dtype": "torch.float16",
            },
            "derived_model_defaults": {"max_model_len": 15000},
        }
        with (
            mock.patch.object(
                dump_vllm_defaults,
                "_optional_import",
                optional_import_from({}),
            ),
            mock.patch.dict(os.environ, {"VLLM_ATTENTION_BACKEND": "ENV_ATTN"}, clear=False),
        ):
            fallback_values = dump_vllm_defaults._aggregate_effective_serve_parameters(
                parsed_cli={},
                engine_args={},
                effective_cfg=fallback_cfg,
                usage_context="OPENAI_API_SERVER",
            )
        self.assertEqual(fallback_values["model"], "fallback-model")
        self.assertEqual(fallback_values["served_model_name"], "fallback-served")
        self.assertEqual(fallback_values["tensor_parallel_size"], 9)
        self.assertEqual(fallback_values["max_model_len"], 15000)
        self.assertEqual(fallback_values["attention_backend"], "ENV_ATTN")
        self.assertEqual(
            fallback_values["_sources"]["attention_backend"],
            "pid_process.environ.VLLM_ATTENTION_BACKEND",
        )
        self.assertFalse(fallback_values["flashinfer_present"])
        self.assertEqual(fallback_values["_effective_mode"], "fallback")

        unavailable = dump_vllm_defaults._aggregate_effective_serve_parameters(
            parsed_cli=None,
            engine_args=None,
            effective_cfg=None,
            usage_context="OPENAI_API_SERVER",
        )
        self.assertIsNone(unavailable["model"])
        self.assertEqual(unavailable["_effective_mode"], "unavailable")

    def test_metadata_parse_args_and_main_output_paths(self) -> None:
        fake_import = optional_import_from(
            {
                "vllm": mod("vllm", __version__="1.2.3", __file__="/tmp/vllm.py"),
                "torch": mod("torch", __version__="2.8.0"),
            }
        )
        args = argparse.Namespace(
            pid=123,
            redact_pid_env=False,
            model_path_override="/models/snapshot",
        )
        with mock.patch.object(dump_vllm_defaults, "_optional_import", fake_import):
            metadata = dump_vllm_defaults._build_metadata(args)
        self.assertEqual(metadata["vllm_version"], "1.2.3")
        self.assertEqual(metadata["vllm_module_path"], "/tmp/vllm.py")
        self.assertEqual(metadata["torch_version"], "2.8.0")
        self.assertEqual(
            metadata["options"],
            {
                "pid": 123,
                "redact_pid_env": False,
                "model_path_override": "/models/snapshot",
            },
        )

        with mock.patch.object(
            sys,
            "argv",
            [
                "dump",
                "--pid",
                "77",
                "--out",
                "x.json",
                "--indent",
                "0",
                "--no-redact-pid-env",
                "--effective-timeout-seconds",
                "12.5",
                "--model-path-override",
                "/models/snapshot",
            ],
        ):
            parsed = dump_vllm_defaults.parse_args()
        self.assertEqual(parsed.pid, 77)
        self.assertEqual(parsed.out, "x.json")
        self.assertEqual(parsed.indent, 0)
        self.assertFalse(parsed.redact_pid_env)
        self.assertEqual(parsed.effective_timeout_seconds, 12.5)
        self.assertEqual(parsed.model_path_override, "/models/snapshot")

        def run_main_with(
            args: argparse.Namespace,
            errors: dict[str, str],
            pid_cwd: str = "/srv/vllm",
            chdir_on_apply: bool = False,
        ) -> tuple[int, str, list[str]]:
            events: list[str] = []

            def apply_env(env: dict[str, str]) -> list[str]:
                _ = env
                events.append("env")
                return ["VLLM_ATTENTION_BACKEND"]

            def apply_cwd(cwd: str) -> str:
                events.append("cwd")
                if chdir_on_apply:
                    os.chdir(cwd)
                    return os.getcwd()
                return cwd

            def force_platform(env: dict[str, str], err: dict[str, str]) -> None:
                _ = env
                events.append("platform")
                err.update(errors)

            def parse_cli(
                raw_cli_args: list[str],
                err: dict[str, str],
                parse_timeout_seconds: float | None = None,
            ) -> tuple[dict[str, str], argparse.Namespace]:
                _ = raw_cli_args, err, parse_timeout_seconds
                events.append("parse")
                return {"model": "m"}, argparse.Namespace(model="m")

            with (
                mock.patch.object(dump_vllm_defaults, "parse_args", return_value=args),
                mock.patch.object(dump_vllm_defaults, "_read_pid_cmdline", return_value=["vllm", "m"]),
                mock.patch.object(dump_vllm_defaults, "_read_pid_environ", return_value={"VLLM_ATTENTION_BACKEND": "X"}),
                mock.patch.object(dump_vllm_defaults, "_read_pid_cwd", return_value=pid_cwd),
                mock.patch.object(dump_vllm_defaults, "_apply_pid_environment", side_effect=apply_env),
                mock.patch.object(dump_vllm_defaults, "_apply_pid_cwd", side_effect=apply_cwd),
                mock.patch.object(dump_vllm_defaults, "_force_platform_if_needed", side_effect=force_platform),
                mock.patch.object(dump_vllm_defaults, "_parse_vllm_cli_input", side_effect=parse_cli),
                mock.patch.object(dump_vllm_defaults, "_extract_cli_defaults", return_value={"cli": True}),
                mock.patch.object(dump_vllm_defaults, "_extract_config_defaults", return_value={"config": True}),
                mock.patch.object(dump_vllm_defaults, "_extract_engine_args_defaults", return_value={"engine": True}),
                mock.patch.object(dump_vllm_defaults, "_extract_effective_engine_config", return_value={"effective": True}),
                mock.patch.object(dump_vllm_defaults, "_aggregate_effective_serve_parameters", return_value={"serve": True}),
                mock.patch.object(dump_vllm_defaults, "_build_metadata", return_value={"metadata": True}),
                contextlib.redirect_stdout(io.StringIO()) as stdout,
            ):
                code = dump_vllm_defaults.main()
            return code, stdout.getvalue(), events

        code, output, events = run_main_with(
            argparse.Namespace(pid=1, redact_pid_env=True, out="-", indent=2, fail_on_error=False),
            {},
        )
        self.assertEqual(code, 0)
        self.assertIn('"pid_process"', output)
        self.assertIn('"effective_serve_parameters"', output)
        result = json.loads(output)
        self.assertEqual(result["pid_process"]["cwd"], "/srv/vllm")
        self.assertEqual(result["pid_process"]["cwd_applied"], "/srv/vllm")
        self.assertEqual(events, ["env", "cwd", "platform", "parse"])

        code, output, _ = run_main_with(
            argparse.Namespace(pid=1, redact_pid_env=True, out="-", indent=2, fail_on_error=True),
            {"forced": "error"},
        )
        self.assertEqual(code, 1)
        self.assertIn('"errors"', output)

        with tempfile.TemporaryDirectory() as tmp_dir:
            out_path = Path(tmp_dir) / "nested" / "dump.json"
            code, stdout, _ = run_main_with(
                argparse.Namespace(
                    pid=1,
                    redact_pid_env=False,
                    out=str(out_path),
                    indent=0,
                    fail_on_error=False,
                ),
                {},
            )
            self.assertEqual(code, 0)
            self.assertEqual(stdout, "")
            self.assertTrue(out_path.exists())

        original_cwd = os.getcwd()
        with tempfile.TemporaryDirectory() as tmp_dir:
            caller_dir = Path(tmp_dir) / "caller"
            pid_dir = Path(tmp_dir) / "pid"
            caller_dir.mkdir()
            pid_dir.mkdir()
            os.chdir(caller_dir)
            try:
                code, stdout, _ = run_main_with(
                    argparse.Namespace(
                        pid=1,
                        redact_pid_env=False,
                        out="relative-dump.json",
                        indent=0,
                        fail_on_error=False,
                    ),
                    {},
                    pid_cwd=str(pid_dir),
                    chdir_on_apply=True,
                )
            finally:
                os.chdir(original_cwd)
            self.assertEqual(code, 0)
            self.assertEqual(stdout, "")
            self.assertTrue((caller_dir / "relative-dump.json").exists())
            self.assertFalse((pid_dir / "relative-dump.json").exists())

    def test_main_records_pid_read_and_inference_errors_without_parsing(self) -> None:
        def fake_effective(
            *,
            engine_args_obj: object | None,
            usage_context_name: str,
            errors: dict[str, str],
            warnings: dict[str, str],
            effective_timeout_seconds: float | None = None,
        ) -> object:
            _ = effective_timeout_seconds
            self.assertIsNone(engine_args_obj)
            self.assertEqual(usage_context_name, "OPENAI_API_SERVER")
            errors["effective.engine_args"] = "missing engine args"
            return None

        def run_main(
            *,
            cmdline: list[str] | BaseException,
            environ: dict[str, str],
            parse_cli: mock.Mock,
            cwd: str | BaseException = "/srv/vllm",
        ) -> dict[str, object]:
            read_cmdline = (
                mock.Mock(side_effect=cmdline)
                if isinstance(cmdline, BaseException)
                else mock.Mock(return_value=cmdline)
            )
            read_cwd = (
                mock.Mock(side_effect=cwd)
                if isinstance(cwd, BaseException)
                else mock.Mock(return_value=cwd)
            )
            with (
                mock.patch.object(
                    dump_vllm_defaults,
                    "parse_args",
                    return_value=argparse.Namespace(
                        pid=99,
                        redact_pid_env=True,
                        out="-",
                        indent=2,
                        fail_on_error=False,
                    ),
                ),
                mock.patch.object(dump_vllm_defaults, "_read_pid_cmdline", read_cmdline),
                mock.patch.object(
                    dump_vllm_defaults,
                    "_read_pid_environ",
                    mock.Mock(return_value=environ),
                ),
                mock.patch.object(dump_vllm_defaults, "_read_pid_cwd", read_cwd),
                mock.patch.object(
                    dump_vllm_defaults,
                    "_apply_pid_cwd",
                    mock.Mock(side_effect=lambda value: value),
                ),
                mock.patch.object(
                    dump_vllm_defaults,
                    "_apply_pid_environment",
                    mock.Mock(return_value=["VLLM_ATTENTION_BACKEND"]),
                ),
                mock.patch.object(dump_vllm_defaults, "_force_platform_if_needed"),
                mock.patch.object(dump_vllm_defaults, "_parse_vllm_cli_input", parse_cli),
                mock.patch.object(dump_vllm_defaults, "_extract_cli_defaults", return_value={}),
                mock.patch.object(dump_vllm_defaults, "_extract_config_defaults", return_value={}),
                mock.patch.object(
                    dump_vllm_defaults,
                    "_extract_engine_args_defaults",
                    return_value={},
                ),
                mock.patch.object(
                    dump_vllm_defaults,
                    "_extract_effective_engine_config",
                    side_effect=fake_effective,
                ),
                mock.patch.object(
                    dump_vllm_defaults,
                    "_aggregate_effective_serve_parameters",
                    return_value={"_effective_mode": "unavailable"},
                ),
                mock.patch.object(dump_vllm_defaults, "_build_metadata", return_value={}),
                contextlib.redirect_stdout(io.StringIO()) as stdout,
            ):
                self.assertEqual(dump_vllm_defaults.main(), 0)
            return json.loads(stdout.getvalue())

        parse_cli = mock.Mock(return_value=({"model": "m"}, argparse.Namespace(model="m")))
        result = run_main(
            cmdline=["bash", "-lc", "vllm serve m"],
            environ={"HF_TOKEN": "secret", "VLLM_ATTENTION_BACKEND": "FLASH_ATTN"},
            parse_cli=parse_cli,
            cwd=PermissionError("cwd denied"),
        )
        parse_cli.assert_not_called()
        self.assertEqual(
            result["pid_process"]["environ"]["HF_TOKEN"],
            "<redacted>",
        )
        self.assertIn("PermissionError('cwd denied')", result["warnings"]["pid.cwd"])
        self.assertIn("PermissionError('cwd denied')", result["pid_process"]["cwd_error"])
        self.assertEqual(
            result["errors"]["pid.inferred_cli_args"],
            "unsupported vLLM launch shape: bash",
        )
        self.assertEqual(result["errors"]["effective.engine_args"], "missing engine args")

        parse_cli = mock.Mock(return_value=({"model": "m"}, argparse.Namespace(model="m")))
        result = run_main(
            cmdline=RuntimeError("proc disappeared"),
            environ={},
            parse_cli=parse_cli,
        )
        parse_cli.assert_not_called()
        self.assertNotIn("pid_process", result)
        self.assertIn("RuntimeError('proc disappeared')", result["errors"]["pid.read"])
        self.assertEqual(result["errors"]["effective.engine_args"], "missing engine args")

    def test_main_reports_model_default_resolution_failure_but_keeps_other_defaults(self) -> None:
        @dataclasses.dataclass
        class EngineArgs:
            max_num_seqs: int = 256

        @dataclasses.dataclass
        class AsyncEngineArgs:
            model: str = "default-model"
            max_num_batched_tokens: int | None = None

            @classmethod
            def add_cli_args(cls, parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
                parser.add_argument("--max-num-batched-tokens", type=int, default=None)
                return parser

            @classmethod
            def from_cli_args(cls, parsed_ns: argparse.Namespace) -> "AsyncEngineArgs":
                raise RuntimeError(
                    "Cannot find google/gemma-4-26B-A4B-it in the cached files "
                    "and outgoing traffic has been disabled. To enable model "
                    "look-ups, set local_files_only=False."
                )

        @dataclasses.dataclass
        class CacheConfig:
            gpu_memory_utilization: float = 0.9

        CacheConfig.__module__ = "vllm.config"

        def make_arg_parser(parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
            parser.add_argument("model_tag", nargs="?")
            parser.add_argument("--max-num-seqs", type=int, default=256)
            return parser

        fake_modules = {
            "vllm": mod("vllm", __version__="test-vllm", __file__="/fake/vllm.py"),
            "torch": mod("torch", __version__="test-torch"),
            "vllm.entrypoints.openai.cli_args": mod(
                "vllm.entrypoints.openai.cli_args",
                make_arg_parser=make_arg_parser,
            ),
            "vllm.engine.arg_utils": mod(
                "vllm.engine.arg_utils",
                EngineArgs=EngineArgs,
                AsyncEngineArgs=AsyncEngineArgs,
            ),
            "vllm.config": mod("vllm.config", CacheConfig=CacheConfig),
        }

        with (
            mock.patch.object(
                dump_vllm_defaults,
                "parse_args",
                return_value=argparse.Namespace(
                    pid=42,
                    redact_pid_env=True,
                    out="-",
                    indent=2,
                    fail_on_error=False,
                ),
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "_read_pid_cmdline",
                return_value=["vllm", "serve", "google/gemma-4-26B-A4B-it"],
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "_read_pid_environ",
                return_value={
                    "HF_HUB_OFFLINE": "1",
                    "TRANSFORMERS_OFFLINE": "1",
                    "HF_TOKEN": "secret",
                },
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "_apply_pid_environment",
                return_value=["HF_HUB_OFFLINE", "TRANSFORMERS_OFFLINE"],
            ),
            mock.patch.object(dump_vllm_defaults, "_read_pid_cwd", return_value="/models"),
            mock.patch.object(dump_vllm_defaults, "_apply_pid_cwd", return_value="/models"),
            mock.patch.object(dump_vllm_defaults, "_force_platform_if_needed"),
            mock.patch.object(
                dump_vllm_defaults,
                "_optional_import",
                optional_import_from(fake_modules),
            ),
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(dump_vllm_defaults.main(), 0)

        result = json.loads(stdout.getvalue())
        self.assertEqual(
            result["pid_process"]["inferred_vllm_cli_args"],
            ["google/gemma-4-26B-A4B-it"],
        )
        self.assertEqual(result["pid_process"]["cwd"], "/models")
        self.assertEqual(result["pid_process"]["cwd_applied"], "/models")
        self.assertEqual(result["pid_process"]["environ"]["HF_TOKEN"], "<redacted>")
        self.assertEqual(
            result["cli_defaults"]["serve_make_arg_parser"],
            {"max_num_seqs": 256, "model_tag": None},
        )
        self.assertEqual(
            result["cli_defaults"]["async_engine_add_cli_args"],
            {"max_num_batched_tokens": None},
        )
        self.assertEqual(
            result["config_defaults"]["CacheConfig"],
            {"gpu_memory_utilization": 0.9},
        )
        self.assertEqual(
            result["engine_args_defaults"],
            {
                "EngineArgs": {"max_num_seqs": 256},
                "AsyncEngineArgs": {
                    "model": "default-model",
                    "max_num_batched_tokens": None,
                },
            },
        )
        self.assertNotIn("parsed_cli_from_input", result)
        self.assertNotIn("engine_args_from_input", result)
        self.assertIsNone(result["effective_engine_config"])
        self.assertEqual(
            result["effective_serve_parameters"]["_effective_mode"],
            "unavailable",
        )
        self.assertIn("local_files_only=False", result["errors"]["input_cli_args_parse"])
        self.assertEqual(
            result["errors"]["effective.engine_args"],
            "No parsed engine args available from PID CLI arguments.",
        )
        self.assertNotIn("warnings", result)

    def test_main_uses_model_path_override_to_recover_offline_snapshot(self) -> None:
        snapshot = (
            "/home/bale1/.cache/huggingface/hub/"
            "models--google--gemma-4-26B-A4B-it/snapshots/47b680"
        )

        class UsageContext(enum.Enum):
            OPENAI_API_SERVER = "openai"

        @dataclasses.dataclass
        class ModelConfig:
            model: str
            served_model_name: str
            max_model_len: int = 32768

        @dataclasses.dataclass
        class SchedulerConfig:
            max_num_batched_tokens: int = 8192
            max_num_seqs: int = 256
            enable_chunked_prefill: bool = True

        @dataclasses.dataclass
        class CacheConfig:
            gpu_memory_utilization: float = 0.9
            cache_dtype: str = "auto"
            enable_prefix_caching: bool = True

        @dataclasses.dataclass
        class ParallelConfig:
            tensor_parallel_size: int = 1
            data_parallel_size: int = 1
            pipeline_parallel_size: int = 1

        @dataclasses.dataclass
        class VllmConfig:
            model_config: ModelConfig
            scheduler_config: SchedulerConfig
            cache_config: CacheConfig
            parallel_config: ParallelConfig

        @dataclasses.dataclass
        class AsyncEngineArgs:
            model: str

            @classmethod
            def add_cli_args(cls, parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
                parser.add_argument("--max-num-seqs", type=int, default=256)
                return parser

            @classmethod
            def from_cli_args(cls, parsed_ns: argparse.Namespace) -> "AsyncEngineArgs":
                if parsed_ns.model_tag != snapshot:
                    raise RuntimeError("offline repo id resolution failed")
                return cls(model=parsed_ns.model_tag)

            def create_engine_config(self, **kwargs: object) -> VllmConfig:
                _ = kwargs
                return VllmConfig(
                    model_config=ModelConfig(
                        model=self.model,
                        served_model_name=self.model,
                    ),
                    scheduler_config=SchedulerConfig(),
                    cache_config=CacheConfig(),
                    parallel_config=ParallelConfig(),
                )

        def make_arg_parser(parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
            parser.add_argument("model_tag", nargs="?")
            parser.add_argument("--port", type=int, default=8000)
            return parser

        fake_modules = {
            "vllm": mod("vllm", __version__="test-vllm", __file__="/fake/vllm.py"),
            "torch": mod("torch", __version__="test-torch"),
            "vllm.entrypoints.openai.cli_args": mod(
                "vllm.entrypoints.openai.cli_args",
                make_arg_parser=make_arg_parser,
            ),
            "vllm.engine.arg_utils": mod(
                "vllm.engine.arg_utils",
                AsyncEngineArgs=AsyncEngineArgs,
            ),
            "vllm.config": mod("vllm.config"),
            "vllm.usage.usage_lib": mod(
                "vllm.usage.usage_lib",
                UsageContext=UsageContext,
            ),
        }

        with (
            mock.patch.object(
                dump_vllm_defaults,
                "parse_args",
                return_value=argparse.Namespace(
                    pid=44,
                    redact_pid_env=True,
                    out="-",
                    indent=2,
                    fail_on_error=False,
                    effective_timeout_seconds=45,
                    model_path_override=snapshot,
                ),
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "_read_pid_cmdline",
                return_value=["vllm", "serve", "google/gemma-4-26B-A4B-it"],
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "_read_pid_environ",
                return_value={
                    "HF_HUB_OFFLINE": "1",
                    "TRANSFORMERS_OFFLINE": "1",
                },
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "_apply_pid_environment",
                return_value=["HF_HUB_OFFLINE", "TRANSFORMERS_OFFLINE"],
            ),
            mock.patch.object(dump_vllm_defaults, "_read_pid_cwd", return_value="/models"),
            mock.patch.object(dump_vllm_defaults, "_apply_pid_cwd", return_value="/models"),
            mock.patch.object(dump_vllm_defaults, "_force_platform_if_needed"),
            mock.patch.object(
                dump_vllm_defaults,
                "_optional_import",
                optional_import_from(fake_modules),
            ),
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(dump_vllm_defaults.main(), 0)

        result = json.loads(stdout.getvalue())
        self.assertEqual(
            result["pid_process"]["original_inferred_vllm_cli_args"],
            ["google/gemma-4-26B-A4B-it"],
        )
        self.assertEqual(result["pid_process"]["inferred_vllm_cli_args"], [snapshot])
        self.assertTrue(result["pid_process"]["model_path_override_applied"])
        self.assertEqual(result["parsed_cli_from_input"]["model_tag"], snapshot)
        self.assertEqual(result["engine_args_from_input"]["model"], snapshot)
        self.assertEqual(
            result["effective_serve_parameters"]["_effective_mode"],
            "full_vllm_config",
        )
        self.assertEqual(result["effective_serve_parameters"]["model"], snapshot)
        self.assertEqual(result["effective_serve_parameters"]["max_num_seqs"], 256)
        self.assertNotIn("errors", result)

    def test_main_reports_online_model_network_failure_as_warnings(self) -> None:
        class UsageContext(enum.Enum):
            OPENAI_API_SERVER = "openai"

        @dataclasses.dataclass
        class AsyncEngineArgs:
            model: str
            tensor_parallel_size: int = 1

            @classmethod
            def add_cli_args(cls, parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
                parser.add_argument("--tensor-parallel-size", type=int, default=1)
                return parser

            @classmethod
            def from_cli_args(cls, parsed_ns: argparse.Namespace) -> "AsyncEngineArgs":
                return cls(
                    model=parsed_ns.model_tag,
                    tensor_parallel_size=parsed_ns.tensor_parallel_size,
                )

            def create_engine_config(self, **kwargs: object) -> object:
                raise ConnectError("[Errno -3] Temporary failure in name resolution")

        class ConnectError(Exception):
            pass

        class CpuPlatform:
            device_type = "cpu"

        def make_arg_parser(parser: argparse.ArgumentParser) -> argparse.ArgumentParser:
            parser.add_argument("model_tag", nargs="?")
            parser.add_argument("--tensor-parallel-size", type=int, default=1)
            return parser

        platforms = mod("vllm.platforms", current_platform=None)
        arg_utils = mod(
            "vllm.engine.arg_utils",
            AsyncEngineArgs=AsyncEngineArgs,
            current_platform=None,
        )
        fake_modules = {
            "vllm": mod(
                "vllm",
                __version__="test-vllm",
                __file__="/fake/vllm.py",
                platforms=platforms,
            ),
            "torch": mod("torch", __version__="test-torch"),
            "vllm.entrypoints.openai.cli_args": mod(
                "vllm.entrypoints.openai.cli_args",
                make_arg_parser=make_arg_parser,
            ),
            "vllm.engine": mod("vllm.engine", arg_utils=arg_utils),
            "vllm.engine.arg_utils": arg_utils,
            "vllm.config": mod("vllm.config"),
            "vllm.usage.usage_lib": mod(
                "vllm.usage.usage_lib",
                UsageContext=UsageContext,
            ),
            "vllm.platforms": platforms,
            "vllm.platforms.cpu": mod("vllm.platforms.cpu", CpuPlatform=CpuPlatform),
        }

        with (
            patched_modules(
                {
                    "vllm": fake_modules["vllm"],
                    "vllm.engine": fake_modules["vllm.engine"],
                    "vllm.engine.arg_utils": arg_utils,
                    "vllm.platforms": platforms,
                    "vllm.platforms.cpu": fake_modules["vllm.platforms.cpu"],
                }
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "parse_args",
                return_value=argparse.Namespace(
                    pid=43,
                    redact_pid_env=True,
                    out="-",
                    indent=2,
                    fail_on_error=False,
                ),
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "_read_pid_cmdline",
                return_value=["vllm", "serve", "google/gemma-4-26B-A4B-it"],
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "_read_pid_environ",
                return_value={
                    "HF_HUB_OFFLINE": "0",
                    "TRANSFORMERS_OFFLINE": "0",
                    "HF_TOKEN": "secret",
                },
            ),
            mock.patch.object(
                dump_vllm_defaults,
                "_apply_pid_environment",
                return_value=["HF_HUB_OFFLINE", "TRANSFORMERS_OFFLINE"],
            ),
            mock.patch.object(dump_vllm_defaults, "_read_pid_cwd", return_value="/models"),
            mock.patch.object(dump_vllm_defaults, "_apply_pid_cwd", return_value="/models"),
            mock.patch.object(dump_vllm_defaults, "_force_platform_if_needed"),
            mock.patch.object(
                dump_vllm_defaults,
                "_optional_import",
                optional_import_from(fake_modules),
            ),
            mock.patch.dict(os.environ, {}, clear=False),
            contextlib.redirect_stdout(io.StringIO()) as stdout,
        ):
            self.assertEqual(dump_vllm_defaults.main(), 0)

        result = json.loads(stdout.getvalue())
        expected_error = "ConnectError('[Errno -3] Temporary failure in name resolution')"
        self.assertEqual(
            result["parsed_cli_from_input"],
            {"model_tag": "google/gemma-4-26B-A4B-it", "tensor_parallel_size": 1},
        )
        self.assertEqual(
            result["engine_args_from_input"],
            {"model": "google/gemma-4-26B-A4B-it", "tensor_parallel_size": 1},
        )
        self.assertEqual(result["effective_engine_config"]["_fallback"], "engine_args_from_input")
        self.assertEqual(
            result["effective_serve_parameters"]["_effective_mode"],
            "fallback",
        )
        self.assertEqual(
            result["effective_serve_parameters"]["model"],
            "google/gemma-4-26B-A4B-it",
        )
        self.assertEqual(
            result["warnings"]["effective.create_engine_config"],
            f"create_engine_config fallback: {expected_error}",
        )
        self.assertEqual(
            result["warnings"]["effective.derive_model_defaults"],
            "Failed to recover effective config in fallback mode: "
            f"{expected_error}",
        )
        self.assertNotIn("errors", result)


if __name__ == "__main__":
    unittest.main()
