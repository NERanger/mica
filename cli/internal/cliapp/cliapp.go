package cliapp

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"mica/cli/internal/build"
	"mica/cli/internal/contracts"
	"mica/cli/internal/deploy"
	"mica/cli/internal/graph"
	"mica/cli/internal/testrun"
	"mica/cli/internal/workspace"
)

const usage = `MICA application tooling

Usage:
  mica init [directory]                  create an application workspace
  mica generate [app.toml]               generate contract code and tokens
  mica build [app.toml]                  generate, validate, and build components
  mica test [app.toml]                   run component and application tests
  mica graph [app.toml] [--format ascii|dot]
  mica deploy [app.toml] [--output PATH] [--local] [--host HOST]

Deploy flags:
  --output PATH     write the makeself artifact to PATH
  --local           execute the generated artifact on this machine
  --host HOST       upload the artifact over SSH and execute it
  --ssh-port N      SSH port for --host
  --start-nats      start a local nats-server for --local
`

func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "generate":
		return runGenerate(args[1:])
	case "build":
		return runBuild(args[1:])
	case "test":
		return runTest(args[1:])
	case "graph":
		return runGraph(args[1:])
	case "deploy":
		return runDeploy(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "mica: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

func parse(fs *flag.FlagSet, args []string) error {
	var flagArgs, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flagArgs = append(flagArgs, arg)
		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") {
			continue
		}
		value := fs.Lookup(name)
		if value == nil {
			continue
		}
		if boolean, ok := value.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return fs.Parse(append(flagArgs, positional...))
}

func load(appArg string) (*workspace.Workspace, error) {
	path := appArg
	if path == "" {
		found, err := workspace.Find(".")
		if err != nil {
			return nil, err
		}
		path = found
	}
	return workspace.Load(path)
}

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "mica: %v\n", err)
	return 1
}

func runGenerate(args []string) int {
	fs := newFlagSet("generate")
	if err := parse(fs, args); err != nil {
		return 2
	}
	ws, err := load(fs.Arg(0))
	if err != nil {
		return fail(err)
	}
	if err := contracts.Generate(ws); err != nil {
		return fail(err)
	}
	fmt.Println("[mica] generated contracts")
	return 0
}

func runBuild(args []string) int {
	fs := newFlagSet("build")
	noGenerate := fs.Bool("no-generate", false, "skip contract generation")
	if err := parse(fs, args); err != nil {
		return 2
	}
	ws, err := load(fs.Arg(0))
	if err != nil {
		return fail(err)
	}
	if _, err := build.Run(ws, !*noGenerate); err != nil {
		return fail(err)
	}
	fmt.Println("[mica] build ok")
	return 0
}

func runTest(args []string) int {
	fs := newFlagSet("test")
	noGenerate := fs.Bool("no-generate", false, "skip contract generation")
	if err := parse(fs, args); err != nil {
		return 2
	}
	ws, err := load(fs.Arg(0))
	if err != nil {
		return fail(err)
	}
	if err := testrun.Run(ws, !*noGenerate); err != nil {
		return fail(err)
	}
	fmt.Println("[mica] test ok")
	return 0
}

func runGraph(args []string) int {
	fs := newFlagSet("graph")
	format := fs.String("format", "ascii", "output format: ascii or dot")
	if err := parse(fs, args); err != nil {
		return 2
	}
	ws, err := load(fs.Arg(0))
	if err != nil {
		return fail(err)
	}
	report, err := contracts.Validate(ws)
	if err != nil {
		return fail(err)
	}
	for _, warning := range report.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	if !report.OK() {
		return fail(fmt.Errorf("%s", strings.Join(report.Errors, "\n")))
	}
	switch *format {
	case "ascii":
		write(os.Stdout, graph.ASCII(ws))
	case "dot":
		write(os.Stdout, graph.DOT(ws))
	default:
		return fail(fmt.Errorf("unknown graph format %q", *format))
	}
	return 0
}

func runDeploy(args []string) int {
	fs := newFlagSet("deploy")
	options := deploy.Options{Generate: true}
	fs.StringVar(&options.Output, "output", "", "artifact output path")
	fs.BoolVar(&options.Local, "local", false, "execute the artifact locally")
	fs.StringVar(&options.Host, "host", "", "upload and execute over SSH")
	fs.IntVar(&options.SSHPort, "ssh-port", 0, "SSH port")
	fs.BoolVar(&options.StartNATS, "start-nats", false, "start a local nats-server")
	noGenerate := fs.Bool("no-generate", false, "skip contract generation")
	if err := parse(fs, args); err != nil {
		return 2
	}
	if *noGenerate {
		options.Generate = false
	}
	if options.Local && options.Host != "" {
		return fail(fmt.Errorf("--local and --host are mutually exclusive"))
	}
	if options.StartNATS && !options.Local {
		return fail(fmt.Errorf("--start-nats requires --local"))
	}
	if options.SSHPort != 0 && options.Host == "" {
		return fail(fmt.Errorf("--ssh-port requires --host"))
	}
	ws, err := load(fs.Arg(0))
	if err != nil {
		return fail(err)
	}
	if err := deploy.Run(ws, options); err != nil {
		return fail(err)
	}
	return 0
}

func write(w io.Writer, text string) {
	fmt.Fprint(w, text)
}
