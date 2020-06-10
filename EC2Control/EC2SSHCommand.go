package EC2Control

import (
    "log"
    "golang.org/x/crypto/ssh"
    "io/ioutil"
    "bytes"
    "time"

    "github.com/aws/aws-sdk-go/service/ec2"
)

func EC2SSHConfig(sshKey string, hostKeyString string) *ssh.ClientConfig {

    hostKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKeyString))
    if err != nil {
        log.Fatalf("invalid hostkey: %s", err)
    }

    key, err := ioutil.ReadFile(sshKey)
    if err != nil {
        log.Fatalf("unable to read private key: %v", err)
    }

    signer, err := ssh.ParsePrivateKey(key)
    if err != nil {
        log.Fatalf("unable to parse private key: %v", err)
    }

    config := &ssh.ClientConfig{
        User: "ec2-user",
        Auth: []ssh.AuthMethod{
            ssh.PublicKeys(signer),
        },
        HostKeyCallback: ssh.FixedHostKey(hostKey),
    }

    return config
}

func EC2SSHCommand(config *ssh.ClientConfig, instance *ec2.Instance, sshCommand string) (string, error) {
    var b bytes.Buffer
    var err error
    var client *ssh.Client
    var retry int = 3

    for retry > 0 {
        client, err = ssh.Dial("tcp", *instance.PublicDnsName + ":22", config)
        if err != nil {
            time.Sleep(3000 * time.Millisecond)
        } else {
            break
        }
        retry--
    }

    if err != nil {
        return "", err
    }

    defer client.Close()

    session, err := client.NewSession()
    if err != nil {
        return "", err
    }
    defer session.Close()
    session.Stdout = &b

    err = session.Run(sshCommand)
    if err != nil {
        return "", err
    }

    return b.String(), nil
}