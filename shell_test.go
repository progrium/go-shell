package shell

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCmdRun(t *testing.T) {
	output := Cmd("echo", "foobar").Run().String()
	if output != "foobar" {
		t.Fatal("output not expected:", output)
	}
}

func TestRun(t *testing.T) {
	output := Run("echo", "foobar").String()
	if output != "foobar" {
		t.Fatal("output not expected:", output)
	}
}

func TestStartWait(t *testing.T) {
	p := Start("echo", "foobar")
	err := p.Wait()
	if err != nil {
		t.Fatal("error not expected:", err)
	}
	output := p.String()
	if output != "foobar" {
		t.Fatal("output not expected:", output)
	}
}

func TestStartKillWait(t *testing.T) {
	p := Start("cat")
	err := p.Kill()
	if err != nil {
		t.Fatal("error not expected:", err)
	}

	var (
		done  = make(chan interface{}, 0)
		timer = time.NewTimer(2 * time.Second)
	)
	go func() {
		p.Wait()
		done <- struct{}{}
	}()
	select {
	case <-timer.C:
		t.Fatal("kill timeout reached")
	case <-done:
		break
	}
}

func TestPanic(t *testing.T) {
	defer func() {
		p := recover().(*Process).ExitStatus
		if p != 2 {
			t.Fatal("status not expected:", p)
		}
	}()
	Run("exit", "2")
}

func TestPipe(t *testing.T) {
	p := Cmd("echo", "foobar").Pipe("wc", "-c").Pipe("awk", "'{print $1}'").Run()
	if p.String() != "7" {
		t.Fatal("output not expected:", p.String())
	}
}

func TestSingleArg(t *testing.T) {
	p := Run("echo foobar | wc -c | awk '{print $1}'")
	if p.String() != "7" {
		t.Fatal("output not expected:", p.String())
	}
}

func TestProcessAsArg(t *testing.T) {
	p := Cmd("echo", Run("echo foobar")).Pipe("wc", "-c").Pipe(Run("echo", "awk"), "'{print $1}'").Run()
	if p.String() != "7" {
		t.Fatal("output not expected:", p.String())
	}
}

func TestLastArgStdin(t *testing.T) {
	p := Cmd("awk '{print $1}'", Cmd("wc", "-c", Cmd("echo foobar"))).Run()
	if p.String() != "7" {
		t.Fatal("output not expected:", p.String())
	}
}

func TestCmdRunFunc(t *testing.T) {
	echo := Cmd("echo").ProcFn()
	output := echo("foobar").String()
	if output != "foobar" {
		t.Fatal("output not expected:", output)
	}
}

func TestPath(t *testing.T) {
	p := Path("/root", "part1/part2", "foobar")
	if p != "/root/part1/part2/foobar" {
		t.Fatal("path not expected:", p)
	}
}

func TestPathTemplate(t *testing.T) {
	tmpl := PathTemplate("/root", "%s/part2", "%s")
	p := tmpl("one", "two")
	if p != "/root/one/part2/two" {
		t.Fatal("path not expected:", p)
	}
}

func TestPrintlnStringer(t *testing.T) {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, Run("echo foobar"))
	if buf.String() != "foobar\n" {
		t.Fatal("output not expected:", buf.String())
	}
}

func TestWrapPanicToErr(t *testing.T) {
	copy := func(src, dst string) (err error) {
		defer func() {
			if p := recover().(*Process); p != nil {
				err = p.Error()
			}
		}()
		Run("cp", src, dst)
		return
	}
	err := copy("", "")
	if !strings.HasPrefix(err.Error(), "[64] ") {
		t.Fatal("output not expected:", err)
	}
}

func TestCmdOutputFn(t *testing.T) {
	copy := Cmd("cp").OutputFn()
	echo := Cmd("echo").OutputFn()
	_, err := copy("", "")
	if !strings.HasPrefix(err.Error(), "[64] ") {
		t.Fatal("output not expected:", err)
	}
	out, err := echo("foobar")
	if err != nil {
		t.Fatal("unexpected error:", err)
	}
	if out != "foobar" {
		t.Fatal("output not expected:", out)
	}
}

func TestSetWorkDir(t *testing.T) {
	var (
		pwd      = os.Getenv("PWD")
		testPath = Path(pwd, "_goshell_unique")
	)
	defer func() {
		os.RemoveAll(Path(pwd, "testfile"))
		os.RemoveAll(testPath)
	}()
	if err := os.MkdirAll(testPath, os.ModePerm); err != nil {
		t.Fatal("unexpected error:", err)
	}

	p := Cmd("touch", "testfile").SetWorkDir(testPath).Run()
	if p.ExitStatus != 0 {
		t.Fatal("unexpected error:", p.Error())
	}

	_, err := os.Stat(Path(testPath, "testfile"))
	if err != nil {
		t.Errorf("expected touched file to be present in correct working dir but was not")
	}
}

func TestCmdTee(t *testing.T) {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		p := Cmd("echo", "test").Tee(pw).Run()
		if p.ExitStatus != 0 {
			t.Fatal(p.Error())
		}

		if p.String() != "test" {
			t.Errorf("expected String() output to be (test), but was (%s)", string(p.String()))
		}
	}()

	out, err := ioutil.ReadAll(pr)
	if err != nil {
		t.Fatal(err)
	}
	pr.Close()

	if string(out) != "test\n" {
		t.Errorf("expected Tee output to be (test\\n), but was (%s)", string(out))
	}
}
