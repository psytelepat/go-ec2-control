package EC2Control

import (
    "fmt"
    "log"

    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ec2"
    "github.com/aws/aws-sdk-go/aws/awserr"
)

type EC2Controller struct {
    Region string
    SVC *ec2.EC2
    Instances []*ec2.Instance
}

func New(region string) EC2Controller {
    var controller EC2Controller = EC2Controller{ Region : region }
    controller.Init();

    return controller
}

func (iw *EC2Controller) Init() {
    session.Must(session.NewSessionWithOptions(session.Options{
        SharedConfigState: session.SharedConfigEnable,
    }))

    iw.SVC = ec2.New(session.New(&aws.Config{
        Region: aws.String(iw.Region),
    }))

    iw.GetInstances()
}

func (iw *EC2Controller) GetInstances() {
    result, err := iw.SVC.DescribeInstances(&ec2.DescribeInstancesInput{})
    if err != nil {
        if aerr, ok := err.(awserr.Error); ok {
            switch aerr.Code() {
            default:
                log.Println(aerr.Error())
            }
        } else {
            log.Println(err.Error())
        }
        return
    }

    iw.Instances = result.Reservations[0].Instances;
}

func (iw *EC2Controller) PrintInstances() {
    for i, instance := range(iw.Instances) {
        fmt.Printf("%10d : \033[1;36m%s\033[0m %s [%s]\n", i, *instance.KeyName, *instance.InstanceId, *instance.State.Name)
    }
}

func (iw *EC2Controller) PrintInstanceInfo(instance *ec2.Instance) {
    fmt.Printf("ID: %s\n", *instance.InstanceId)
    fmt.Printf("Key Name: %s\n", *instance.KeyName)

    var statusCode int64 = *instance.State.Code
    var statusCodeName string = *instance.State.Name
    fmt.Printf("Status: %s (%d)\n", statusCodeName, statusCode)

    if instance.PublicDnsName != nil && len(*instance.PublicDnsName) > 0 {
        fmt.Printf("Host: %s\n", *instance.PublicDnsName)
    }
    
    if instance.PublicIpAddress != nil && len(*instance.PublicIpAddress) > 0 {
        fmt.Printf("IP: %s\n", *instance.PublicIpAddress)
    }
}

func (iw *EC2Controller) SelectInstanceById(InstanceId string) *ec2.Instance {
    for _, instance := range(iw.Instances) {
        if *instance.InstanceId == InstanceId {
            return instance
        }
    }
    return nil
}

func (iw *EC2Controller) SelectInstance(auto bool) *ec2.Instance {
    
    if len(iw.Instances) == 0 {
        return nil
    }

    if auto && len(iw.Instances) == 1 {
        return iw.Instances[0]
    }

    var i int
    for {
        fmt.Println("Select instance:");
    
        iw.PrintInstances()

        fmt.Print("> Number: ");
        
        _, err := fmt.Scanf("%d\n", &i)
        
        if err != nil {
            fmt.Println("Invalid choice:", err);
        } else if 0 > i || i >= len(iw.Instances) {
            fmt.Println("Invalid choice: out of range");
        } else {
            break
        }
    }

    return iw.Instances[i]
}

func (iw *EC2Controller) StartInstance(instance *ec2.Instance) {
    input := &ec2.StartInstancesInput{
        InstanceIds: []*string{
            aws.String(*instance.InstanceId),
        },
    }

    _, err := iw.SVC.StartInstances(input)
    if err != nil {
        if aerr, ok := err.(awserr.Error); ok {
            switch aerr.Code() {
            default:
                log.Println(aerr.Error())
            }
        } else {
            log.Println(err.Error())
        }
    }
}

func (iw *EC2Controller) StopInstance(instance *ec2.Instance) {
    input := &ec2.StopInstancesInput{
        InstanceIds: []*string{
            aws.String(*instance.InstanceId),
        },
    }

    _, err := iw.SVC.StopInstances(input)
    if err != nil {
        if aerr, ok := err.(awserr.Error); ok {
            switch aerr.Code() {
            default:
                log.Println(aerr.Error())
            }
        } else {
            log.Println(err.Error())
        }
    }
}