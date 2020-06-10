package main

import (
    "log"
    "fmt"
    "os"
    "bufio"
    "io/ioutil"
    "strings"
    "time"
    "os/exec"
    "os/signal"
    "syscall"
    "context"

    "github.com/joho/godotenv"
    "go-ec2-control/EC2Control"
    "github.com/aws/aws-sdk-go/service/ec2"
)

var selectedInstance *ec2.Instance
var exit_chan chan int

var commands []string = []string{
    "help    : this list",
    "select  : select instance",
    "ls      : list instances",
    "upd     : refresh instances list",
    "info    : get info on #SI#",
    "start   : start #SI#",
    "stop    : stop #SI#",
    "run     : start ovpn daemon on #SI#",
    "file    : get ovpn file from #SI#",
    "ssh     : run ssh command on #SI#",
    "set     : install and connect to ovpn server on #SI#",
    "up      : connect to vpn",
    "down    : disconnect from vpn",
    "vpn     : start + run + file + set + up",
    "exit    : quit program",
}

func Help() {
    fmt.Println("\nEC2 Control")
    fmt.Println(strings.Repeat("----------", 6))
    for _, command := range(commands) {
        fmt.Println(strings.ReplaceAll(command,"#SI#",fmt.Sprintf("\033[1;36m%s\033[0m",*selectedInstance.KeyName)))
    }
}

func scanInput() string {
    input, _ := bufio.NewReader(os.Stdin).ReadString('\n')
    input = strings.Trim(input, "\n")
    return input
}

func init() {
    exit_chan = make(chan int)

    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }
}

func mainThread(ctx context.Context) {
    sshKeyFile, exists := os.LookupEnv("SSH_KEY_FILE")
    if !exists {
        log.Fatalf("no SSH_KEY_FILE env variable")
        return
    }

    sshHostKey, exists := os.LookupEnv("SSH_HOST_KEY")
    if !exists {
        log.Fatalf("no SSH_HOST_KEY env variable")
        return
    }

    awsRegion, exists := os.LookupEnv("AWS_REGION")
    if !exists {
        log.Fatalf("no AWS_REGION env variable")
        return
    }

    dockerVolume, exists := os.LookupEnv("DOCKER_VOLUME")
    if !exists {
        log.Fatalf("no DOCKER_VOLUME env variable")
        return
    }

    ovpnHost, exists := os.LookupEnv("OVPN_HOST")
    if !exists {
        log.Fatalf("no OVPN_HOST env variable")
        return
    }

    ovpnUser, _ := os.LookupEnv("OVPN_USER")

    nmcliExecutable, exists := os.LookupEnv("NMCLI")
    if !exists {
        log.Fatalf("no NMCLI env variable")
        return
    }

    cwd, err := os.Getwd()
    if err != nil {        
        log.Fatalf("failed to get working dir")
        return
    }

    var (
        ctrl EC2Control.EC2Controller = EC2Control.New(awsRegion)
        cmd string
        shellCmd *exec.Cmd
        queue []string
    )

    sshConfig := EC2Control.EC2SSHConfig(sshKeyFile, sshHostKey)
    
    selectedInstance = ctrl.SelectInstance(true)

    Help()

    if selectedInstance != nil {
        fmt.Printf("\nSelected instance: \033[1;36m%s\033[0m %s\n", *selectedInstance.KeyName, *selectedInstance.InstanceId)
    }

    for {
        select {
            case <-ctx.Done():
                return
            default:
                fmt.Print("\n> ")
                _, err := fmt.Scanf("%s\n", &cmd)
                
                if err != nil {
                    fmt.Println("Invalid command:", err)
                    continue
                }

                queue = append(queue, cmd)

                for len(queue) > 0 {
                    cmd = queue[0]
                    queue = queue[1:]

                    switch cmd {
                        case "h":
                            fallthrough
                        case "help":
                            Help()
                        case "s":
                            fallthrough
                        case "select":
                            selectedInstance = ctrl.SelectInstance(false)
                            fmt.Printf("Selected instance: \033[1;36m%s\033[0m %s\n", *selectedInstance.KeyName, *selectedInstance.InstanceId)
                        case "list":
                            fallthrough
                        case "ls":
                            fmt.Println("Instances list:")
                            ctrl.PrintInstances()
                        case "u":
                            fallthrough
                        case "upd":
                            ctrl.GetInstances()
                            selectedInstance = ctrl.SelectInstanceById(*selectedInstance.InstanceId)
                            ctrl.PrintInstances()
                        case "i":
                            fallthrough
                        case "info":
                            ctrl.PrintInstanceInfo(selectedInstance)
                        case "start":
                            if selectedInstance != nil && *selectedInstance.State.Code == 16 {
                                fmt.Println("Already running.")
                                continue
                            }

                            ctrl.StartInstance(selectedInstance)
                            fmt.Printf("Starting instance: %s...", *selectedInstance.InstanceId)

                            for {
                                time.Sleep(5000 * time.Millisecond)
                                fmt.Print(".")
                                ctrl.GetInstances()
                                selectedInstance = ctrl.SelectInstanceById(*selectedInstance.InstanceId)
                                if selectedInstance != nil && *selectedInstance.State.Code == 16 {
                                    break
                                }
                            }

                            fmt.Println("Done.")
                        case "stop":
                            if selectedInstance != nil && *selectedInstance.State.Code == 80 {
                                fmt.Println("Already stopped.")
                                continue
                            }

                            ctrl.StopInstance(selectedInstance)
                            fmt.Printf("Stopping instance: %s...", *selectedInstance.InstanceId)

                            for {
                                time.Sleep(5000 * time.Millisecond)
                                fmt.Print(".")
                                ctrl.GetInstances()
                                selectedInstance = ctrl.SelectInstanceById(*selectedInstance.InstanceId)
                                if selectedInstance != nil && *selectedInstance.State.Code == 80 {
                                    break
                                }
                            }

                            fmt.Println("Done.")
                        case "run":
                            var volumeName string

                            if dockerVolume == "" {
                                fmt.Print("> Docker volume name: ")
                                volumeName = scanInput()
                            } else {
                                volumeName = dockerVolume
                            }

                            command := fmt.Sprintf("docker run -v " + volumeName + ":/etc/openvpn -d -p 1194:1194/udp --cap-add=NET_ADMIN kylemanna/openvpn")
                            output, err := EC2Control.EC2SSHCommand(sshConfig,selectedInstance,command)
                            if err != nil {
                                fmt.Println("Error:", err)
                            } else {
                                fmt.Println(output)
                            }
                        case "file":
                            var username string

                            if ovpnUser == "" {
                                fmt.Print("> OVPN username: ")
                                username = scanInput()
                            } else {
                                username = ovpnUser
                            }

                            var volumeName string

                            if dockerVolume == "" {
                                fmt.Print("> Docker volume name: ")
                                volumeName = scanInput()
                            } else {
                                volumeName = dockerVolume
                            }

                            command := fmt.Sprintf("docker run -v " + volumeName + ":/etc/openvpn --log-driver=none --rm kylemanna/openvpn ovpn_getclient %s", username)
                            output, err := EC2Control.EC2SSHCommand(sshConfig,selectedInstance,command)
                            if err != nil {
                                fmt.Println("Error:", err)
                            } else {
                                output = strings.ReplaceAll(output, ovpnHost, *selectedInstance.PublicIpAddress);
                                err := ioutil.WriteFile(ovpnHost + ".ovpn", []byte(output), 0644)
                                if err != nil {
                                    fmt.Println("Error:", err)
                                } else {
                                    fmt.Println("File written.")
                                }
                            }
                        case "set":
                            shellCmd = exec.Command(nmcliExecutable, "connection", "delete", ovpnHost)
                            fmt.Printf("Deleting previous %s config...", ovpnHost)
                            shellCmd.Run()
                            fmt.Println("Done.")

                            shellCmd = exec.Command(nmcliExecutable, "connection", "import", "type", "openvpn", "file",  cwd + "/" + ovpnHost + ".ovpn")
                            fmt.Printf("Setting up new config for %s...", ovpnHost)
                            err = shellCmd.Run()
                            if err != nil {
                                fmt.Println("Failed.")
                            } else {
                                fmt.Println("Done.")
                            }
                        case "up":
                            shellCmd = exec.Command(nmcliExecutable, "connection", "up", ovpnHost)
                            fmt.Printf("Connecting to %s...", ovpnHost)
                            err = shellCmd.Run()
                            if err != nil {
                                fmt.Println("Failed.")
                            } else {
                                fmt.Println("Done.")
                            }
                        case "down":
                            shellCmd = exec.Command(nmcliExecutable, "connection", "down", ovpnHost)
                            fmt.Printf("Stopping %s...", ovpnHost)
                            shellCmd.Run()
                            fmt.Println("Done.")
                        case "ssh":
                            fmt.Print("> SSH command: ")
                            command := scanInput()
                            output, err := EC2Control.EC2SSHCommand(sshConfig,selectedInstance,command)
                            if err != nil {
                                fmt.Println("Error:", err)
                            } else {
                                fmt.Println(output)
                            }
                        case "vpn":
                            queue = append(queue, "start", "run", "file", "set", "up")
                        break
                        case "quit":
                            fallthrough
                        case "x":
                            fallthrough
                        case "q":
                            fallthrough
                        case "exit":
                            fmt.Println("Bye! C ya l8er!")
                            exit_chan <- 3
                            return
                        default:
                            fmt.Println("Invalid command")
                    }
                }
        }
    }
}

func sigThread() {
    signal_chan := make(chan os.Signal, 1)
    signal.Notify(signal_chan,
        syscall.SIGHUP,
        syscall.SIGINT,
        syscall.SIGTERM,
        syscall.SIGQUIT)

    select {
        case s := <-signal_chan:
            switch s {
                case syscall.SIGHUP:
                    exit_chan <- 1
                case syscall.SIGINT:
                    exit_chan <- 2
                case syscall.SIGTERM:
                    exit_chan <- 15
                case syscall.SIGQUIT:
                    exit_chan <- 3
                default:
                    exit_chan <- 3
            }
        break
    }
}

func main(){
    go sigThread()

    ctx, stopMainThread := context.WithCancel(context.Background())
    go mainThread(ctx)

    code := <-exit_chan
    
    stopMainThread()

    fmt.Print("\n\n")
    os.Exit(code)
}
