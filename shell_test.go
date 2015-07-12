package shell

import "testing"

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

func TestCmdFunc(t *testing.T) {
	echo := Cmd("echo").Func()
	output := echo("foobar").String()
	if output != "foobar" {
		t.Fatal("output not expected:", output)
	}
}
