package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	Shell = "/bin/sh"
	Panic = true
	Trace = false

	exit = os.Exit
)

func assert(err error) {
	if err != nil {
		panic(err)
	}
}

func Path(parts ...string) string {
	return filepath.Join(parts...)
}

func PathTemplate(parts ...string) func(...interface{}) string {
	return func(values ...interface{}) string {
		return fmt.Sprintf(Path(parts...), values...)
	}
}

func ErrExit() {
	if p, ok := recover().(*Process); p != nil {
		if !ok {
			fmt.Fprintf(os.Stderr, "Unexpected panic: %v\n", p)
			exit(1)
		}
		fmt.Fprintf(os.Stderr, "%s\n", p.Error())
		exit(p.ExitStatus)
	}
}

type Command struct {
	cmd []string
	in  *Command
}

func (c *Command) ProcFn() func(...interface{}) *Process {
	return func(args ...interface{}) *Process {
		c.cmd = append(c.cmd, c.args(args...)...)
		return c.Run()
	}
}

func (c *Command) OutputFn() func(...interface{}) (string, error) {
	return func(args ...interface{}) (out string, err error) {
		c.cmd = append(c.cmd, c.args(args...)...)
		defer func() {
			if p, ok := recover().(*Process); p != nil {
				if ok {
					err = p.Error()
				} else {
					err = fmt.Errorf("panic: %v", p)
				}
			}
		}()
		out = c.Run().String()
		return
	}
}

func (c *Command) ErrFn() func(...interface{}) error {
	return func(args ...interface{}) (err error) {
		c.cmd = append(c.cmd, c.args(args...)...)
		defer func() {
			if p, ok := recover().(*Process); p != nil {
				if ok {
					err = p.Error()
				} else {
					err = fmt.Errorf("panic: %v", p)
				}
			}
		}()
		c.Run()
		return
	}
}

func (c *Command) Pipe(cmd ...interface{}) *Command {
	return Cmd(append(cmd, c)...)
}

func (c *Command) args(args ...interface{}) []string {
	var strArgs []string
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			strArgs = append(strArgs, v)
		case fmt.Stringer:
			strArgs = append(strArgs, v.String())
		default:
			cmd, ok := arg.(*Command)
			if i+1 == len(args) && ok {
				c.in = cmd
				continue
			}
			panic("invalid type for argument")
		}
	}
	return strArgs
}

func (c *Command) Run() *Process {
	shellCmd := strings.Join(c.cmd, " ")
	if Trace {
		fmt.Fprintln(os.Stderr, "+", shellCmd)
	}
	cmd := exec.Command(Shell, "-c", shellCmd)
	p := new(Process)
	if c.in != nil {
		cmd.Stdin = c.in.Run()
	} else {
		stdin, err := cmd.StdinPipe()
		assert(err)
		p.Stdin = stdin
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	p.Stdout = &stdout
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	p.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if stat, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				p.ExitStatus = int(stat.ExitStatus())
				if Panic {
					panic(p)
				}
			}
		} else {
			assert(err)
		}
	}
	return p
}

func Cmd(cmd ...interface{}) *Command {
	c := new(Command)
	c.cmd = c.args(cmd...)
	return c
}

type Process struct {
	Stdout     *bytes.Buffer
	Stderr     *bytes.Buffer
	Stdin      io.WriteCloser
	ExitStatus int
}

func (p *Process) String() string {
	return strings.Trim(p.Stdout.String(), "\n")
}

func (p *Process) Bytes() []byte {
	return p.Stdout.Bytes()
}

func (p *Process) Error() error {
	return fmt.Errorf("%s (%v)", strings.Trim(p.Stderr.String(), "\n"), p.ExitStatus)
}

func (p *Process) Read(b []byte) (int, error) {
	return p.Stdout.Read(b)
}

func (p *Process) Write(b []byte) (int, error) {
	return p.Stdin.Write(b)
}

func Run(cmd ...interface{}) *Process {
	return Cmd(cmd...).Run()
}
