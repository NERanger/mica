from __future__ import annotations

import argparse
import sys
from pathlib import Path

from mica.cli.graph import graph_ascii, graph_dot
from mica.cli.inspect import render_inspect
from mica.cli.manifest import ManifestError
from mica.cli.run import run_app
from mica.cli.validate import validate_app


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="mica", description="MICA application tooling")
    sub = parser.add_subparsers(dest="command", required=True)

    validate = sub.add_parser("validate", help="validate an application manifest")
    validate.add_argument("app_toml")
    validate.add_argument("--image", type=Path, default=None)

    inspect = sub.add_parser("inspect", help="print static application topology")
    inspect.add_argument("app_toml")
    inspect.add_argument("--format", choices=("text", "json"), default="text")
    inspect.add_argument("--image", type=Path, default=None)

    graph = sub.add_parser("graph", help="print a communication graph")
    graph.add_argument("app_toml")
    graph.add_argument("--format", choices=("ascii", "dot"), default="ascii")
    graph.add_argument("--image", type=Path, default=None)

    run = sub.add_parser("run", help="launch a local application")
    run.add_argument("app_toml")
    run.add_argument("--start-nats", action="store_true")
    run.add_argument("--image", type=Path, default=None)
    return parser


def _print_result(errors: list[str], warnings: list[str]) -> None:
    for warning in warnings:
        print(f"warning: {warning}", file=sys.stderr)
    for error in errors:
        print(error, file=sys.stderr)


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    app_path = Path(args.app_toml)
    try:
        result = validate_app(app_path, image=args.image)
    except ManifestError as exc:
        print(str(exc), file=sys.stderr)
        return 1
    if args.command == "validate":
        _print_result(result.errors, result.warnings)
        if result.ok:
            print(f"ok: {app_path}")
            return 0
        return 1
    if not result.ok:
        _print_result(result.errors, result.warnings)
        return 1
    if args.command == "inspect":
        _print_result([], result.warnings)
        sys.stdout.write(render_inspect(result, args.format))
        return 0
    if args.command == "graph":
        _print_result([], result.warnings)
        loaded = result.loaded
        assert loaded is not None
        if args.format == "dot":
            sys.stdout.write(graph_dot(loaded))
        else:
            sys.stdout.write(graph_ascii(loaded))
        return 0
    if args.command == "run":
        return run_app(result, start_nats=args.start_nats)
    parser.error("unknown command")
    return 2


if __name__ == "__main__":
    sys.exit(main())
