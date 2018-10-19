package shell

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var (
	Shell       = []string{"/bin/sh", "-c"}
	Panic       = true
	Trace       = false
	TracePrefix = "+"

	exit = os.Exit
)

var Tee io.Writer

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

func Quote(arg string) string {
	return fmt.Sprintf("'%s'", strings.Replace(arg, "'", "'\\''", -1))
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
	args []string
	in   *Command
	wd   string
}

func (c *Command) ProcFn() func(...interface{}) *Process {
	return func(args ...interface{}) *Process {
		cmd := &Command{c.args, c.in, c.wd}
		cmd.addArgs(args...)
		return cmd.Run()
	}
}

func (c *Command) OutputFn() func(...interface{}) (string, error) {
	return func(args ...interface{}) (out string, err error) {
		cmd := &Command{c.args, c.in, c.wd}
		cmd.addArgs(args...)
		defer func() {
			if p, ok := recover().(*Process); p != nil {
				if ok {
					err = p.Error()
				} else {
					err = fmt.Errorf("panic: %v", p)
				}
			}
		}()
		out = cmd.Run().String()
		return
	}
}

func (c *Command) ErrFn() func(...interface{}) error {
	return func(args ...interface{}) (err error) {
		cmd := &Command{c.args, c.in, c.wd}
		cmd.addArgs(args...)
		defer func() {
			if p, ok := recover().(*Process); p != nil {
				if ok {
					err = p.Error()
				} else {
					err = fmt.Errorf("panic: %v", p)
				}
			}
		}()
		cmd.Run()
		return
	}
}

func (c *Command) Pipe(cmd ...interface{}) *Command {
	return Cmd(append(cmd, c)...)
}

func (c *Command) SetWorkDir(path string) *Command {
	c.wd = path
	return c
}

func (c *Command) addArgs(args ...interface{}) {
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
	c.args = append(c.args, strArgs...)
}

func (c *Command) shellCmd(quote bool) string {
	if !quote {
		return strings.Join(c.args, " ")
	}
	var quoted []string
	for i := range c.args {
		quoted = append(quoted, Quote(c.args[i]))
	}
	return strings.Join(quoted, " ")
}

func (c *Command) Run() *Process {
	cmd := exec.Command(Shell[0], append(Shell[1:], c.shellCmd(false))...)
	return c.execute(cmd, cmd.Run)
}

func (c *Command) Start() *Process {
	cmd := exec.Command(Shell[0], append(Shell[1:], c.shellCmd(false))...)
	return c.execute(cmd, cmd.Start)
}

func (c *Command) execute(cmd *exec.Cmd, call func() error) *Process {
	if Trace {
		fmt.Fprintln(os.Stderr, TracePrefix, c.shellCmd(false))
	}
	cmd.Dir = c.wd
	log.Println(cmd.Args)
	p := new(Process)
	p.cmd = cmd
	if c.in != nil {
		cmd.Stdin = c.in.Run()
	} else {
		stdin, err := cmd.StdinPipe()
		assert(err)
		p.Stdin = stdin
	}
	var stdout bytes.Buffer
	if Tee != nil {
		cmd.Stdout = io.MultiWriter(&stdout, Tee)
	} else {
		cmd.Stdout = &stdout
	}
	p.Stdout = &stdout
	var stderr bytes.Buffer
	if Tee != nil {
		cmd.Stderr = io.MultiWriter(&stderr, Tee)
	} else {
		cmd.Stderr = &stderr
	}
	p.Stderr = &stderr
	err := call()
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
	c.addArgs(cmd...)
	return c
}

type Process struct {
	cmd    *exec.Cmd
	killed bool

	Stdout     *bytes.Buffer
	Stderr     *bytes.Buffer
	Stdin      io.WriteCloser
	ExitStatus int
}

func (p *Process) Wait() error {
	err := p.cmd.Wait()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if stat, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				p.ExitStatus = int(stat.ExitStatus())
				if Panic && !p.killed {
					panic(p)
				}
			}
		}
	}
	return err
}

func (p *Process) Kill() error {
	p.killed = true
	err := p.cmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("killed error: %s", err)
	}
	if err := p.Wait(); err == nil {
		if !strings.Contains(err.Error(), "signal: killed") {
			return err
		}
	}
	return nil
}

func (p *Process) String() string {
	return strings.Trim(p.Stdout.String(), "\n")
}

func (p *Process) Bytes() []byte {
	return p.Stdout.Bytes()
}

func (p *Process) Error() error {
	errlines := strings.Split(p.Stderr.String(), "\n")
	s := len(errlines)
	if s > 1 {
		s -= 1
	} else {
		s = 0
	}
	return fmt.Errorf("[%v] %s\n", p.ExitStatus, errlines[s])
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

func Start(cmd ...interface{}) *Process {
	return Cmd(cmd...).Start()
}
