package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func findLineEnd(dat []byte) (out [][]byte) {
	prev := 0;
	for pos,i := range dat {
		if i == []byte("\n")[0] {
			out = append(out, dat[prev:pos]);	
			prev = pos + 1;
		}
	}

	out = append(out, dat[prev:])
	// for pos,i := range out {
	// 	count := 0;
	// 	for _,char := range i {
	// 		if (char == []byte(" ")[0]) {
	// 			count++;
	// 		}
	// 	}
	// 	if count == len(i)  && pos > 0{
	// 		out = append(out[:pos-1],out[pos:]...)
	// 	}
	// }
	return;
}

func copyAndCapture(process string,w io.Writer, r io.Reader) {
	prefix := []byte(fmt.Sprintf("Child process %s):",process) )
	after  := []byte("\n")
    buf := make([]byte, 1024, 1024)
    for {
        n, err := r.Read(buf[:])
        if n > 0 {
            d := buf[:n]
			lines := findLineEnd(d);
			for _,line := range lines {
				out := append(prefix,line...)
				out = append(out,after...)

				_, err := w.Write(out)
				if err != nil {
					return;
				}
			}
        }
        if err != nil {
            // Read returns io.EOF at the end of file, which is not an error for us
            if err == io.EOF {
                err = nil
            }
                return;
        }
    }
}

func HandleProcess(cmd *exec.Cmd) {
	if cmd == nil {
		return;
	}
	processname := cmd.Args[0];

	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()
	cmd.Start();
	go func ()  {
		copyAndCapture(processname,os.Stdout, stdoutIn)
	}();
	go func ()  {
		copyAndCapture(processname,os.Stdout, stderrIn)
	}();
	cmd.Wait();

}

func main() {
	command := "screencoder.exe";
	name 	:= "mumbai"
	url		:= "https://auth.thinkmay.net"
	hidport := 5000
	engine  := "screencoder"


	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--url" {
			url = args[i+1]
		} else if arg == "--name" {
			name = args[i+1]
		} else if arg == "--engine" {
			engine = args[i+1]
		} else if arg == "--help" {
			fmt.Printf("--engine |  encode engine ()\n")
			return
		}
	}





	
	shutdown := make(chan bool)
	var proxy,devsim *exec.Cmd;
	go func ()  {
		for {
			chann := make( chan os.Signal,10) 
			signal.Notify( chann, syscall.SIGTERM , os.Interrupt);
			<-chann;

			if proxy != nil {
				if proxy.Process != nil {
					proxy.Process.Kill()	
				}
			}
			if devsim != nil {
				if devsim.Process != nil {
					devsim.Process.Kill()	
				}
			}
			shutdown<-true;
		}
	}()

	count := 1;
	waitforhid := make(chan bool)
	go func ()  {
		for {
			devsim = exec.Command("DevSim.exe",fmt.Sprintf( "--urls=http://localhost:%d",hidport));
			log := make([]byte,0);
			for _,i := range devsim.Args {
				log = append(log, append([]byte(i),[]byte(" ")...)...);
			}
			fmt.Printf("starting device simulator : %s\n",string(log));

			done := make(chan bool)
			failed := make(chan bool,2)
			success := make(chan bool)
			go func ()  {
				HandleProcess(devsim);
				failed<-true;
				done<-true;
			}()
			go func ()  {
				time.Sleep(2 * time.Second);
				success<-true;
			}()
			go func ()  {
				for {
					select {
					case <-success:
						waitforhid<-true;
						return;
					case <-failed:
						hidport++;
						done<-true;
					}
				}
			}()
			<-done
			count++;
		}	
	} ()

	<-waitforhid
	go func ()  {
		for {
			time.Sleep(2 * time.Second);
			resp,err := http.Get(fmt.Sprintf("%s/auth/server/%s",url,name))
			if err != nil{
				fmt.Printf("%s\n",err.Error());
				continue;
			}

			body := make([]byte,1000);	
			size,err := resp.Body.Read(body);
			if err != nil{
				fmt.Printf("%s\n",err.Error());
				continue;
			}
			token := string(body[:size]);
			if token == "none" {
				fmt.Printf("empty token\n");
				continue;
			}

			proxy = exec.Command(command,
				"--token",token,
				"--hid",fmt.Sprintf("localhost:%d",hidport),
				"--engine",engine);

			log := make([]byte,0);
			for _,i := range proxy.Args {
				log = append(log, append([]byte(i),[]byte(" ")...)...);
			}
			fmt.Printf("starting webrtc proxy: %s\n",string(log));
			HandleProcess(proxy);
		}
	} ();

	<-shutdown
}
