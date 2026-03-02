package main

import (
    "context"
    "fmt"
    "net"
    "os"
    "path"
    "time"
	"encoding/json"

    "google.golang.org/grpc"
    pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
    socketPath   = "/var/lib/kubelet/device-plugins/"
	resourceBaseName = "devices.threax.com/"
)

type DeviceResourcePlugin struct {
	pluginapi.UnimplementedDevicePluginServer
    socket     string
    devices    []*pluginapi.Device
    server     *grpc.Server
    health     chan *pluginapi.Device
	devicePaths []string
	resourceName string
}

func NewDeviceResourcePlugin(name string, devicePaths []string) *DeviceResourcePlugin {
    return &DeviceResourcePlugin{
        socket:  path.Join(socketPath, name + "devices.sock"),
        devices: makek8sDevice(name),
        health:  make(chan *pluginapi.Device),
		resourceName: resourceBaseName + name,
		devicePaths: devicePaths,
    }
}

func makek8sDevice(name string) []*pluginapi.Device {
    devices := []*pluginapi.Device{}

	devices = append(devices, &pluginapi.Device{
		ID:     name,
		Health: pluginapi.Healthy,
	})

    return devices
}

func (p *DeviceResourcePlugin) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
    return &pluginapi.DevicePluginOptions{
        PreStartRequired: false,
    }, nil
}

func (p *DeviceResourcePlugin) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
    // Send initial device list
    if err := stream.Send(&pluginapi.ListAndWatchResponse{Devices: p.devices}); err != nil {
        return err
    }

    // Watch for health changes
    for {
        select {
        case device := <-p.health:
            // Update device health
            for _, dev := range p.devices {
                if dev.ID == device.ID {
                    dev.Health = device.Health
                }
            }
            // Send updated list
            if err := stream.Send(&pluginapi.ListAndWatchResponse{Devices: p.devices}); err != nil {
                return err
            }
			<-time.After(600 * time.Second)
        }
    }
}

func (p *DeviceResourcePlugin) Allocate(ctx context.Context, req *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
    responses := &pluginapi.AllocateResponse{}

    for _, containerReq := range req.ContainerRequests {
		print(containerReq)

        containerResp := &pluginapi.ContainerAllocateResponse{}

        for _, devicePath := range p.devicePaths {
            // Add device to container
            containerResp.Devices = append(containerResp.Devices, &pluginapi.DeviceSpec{
                HostPath:      devicePath,
                ContainerPath: devicePath,
                Permissions:   "rw",
            })
        }

        responses.ContainerResponses = append(responses.ContainerResponses, containerResp)
    }

    return responses, nil
}

func (p *DeviceResourcePlugin) PreStartContainer(ctx context.Context, req *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
    return &pluginapi.PreStartContainerResponse{}, nil
}

func (p *DeviceResourcePlugin) GetPreferredAllocation(ctx context.Context, req *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
    return &pluginapi.PreferredAllocationResponse{}, nil
}

func (p *DeviceResourcePlugin) Register() error {
    conn, err := grpc.Dial(
        pluginapi.KubeletSocket,
        grpc.WithInsecure(),
        grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
            return net.DialTimeout("unix", addr, timeout)
        }),
    )
    if err != nil {
        return err
    }
    defer conn.Close()

    client := pluginapi.NewRegistrationClient(conn)
    request := &pluginapi.RegisterRequest{
        Version:      pluginapi.Version,
        Endpoint:     path.Base(p.socket),
        ResourceName: p.resourceName,
    }

    _, err = client.Register(context.Background(), request)
    return err
}

func (p *DeviceResourcePlugin) Serve() error {
    // Remove old socket if exists
    os.Remove(p.socket)

    listener, err := net.Listen("unix", p.socket)
    if err != nil {
        return err
    }

    p.server = grpc.NewServer()
    pluginapi.RegisterDevicePluginServer(p.server, p)

    go func() {
        p.server.Serve(listener)
    }()

    // Wait for server to start
    time.Sleep(1 * time.Second)

    return p.Register()
}

func (p *DeviceResourcePlugin) Stop() {
    if p.server != nil {
        p.server.Stop()
    }
    os.Remove(p.socket)
}

type ConfigItem struct {
	Name string
	Paths []string
}

func main() {
	fmt.Println("Starting Server...")

	for _, arg := range os.Args[1:] {
		var argData ConfigItem
		err := json.Unmarshal([]byte(arg), &argData)
		if err != nil {
			fmt.Println(arg);
			fmt.Println(err)
			os.Exit(1)
		}

		plugin := NewDeviceResourcePlugin(argData.Name, argData.Paths)

		fmt.Println("Setting up k8s device resource plugin for " + plugin.resourceName);
		for _, path := range plugin.devicePaths {
			fmt.Println("     device path " + path);
		}

		if err := plugin.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start plugin: %v\n", err)
			os.Exit(1)
		}
	}

    fmt.Println("Simple Device Plugin started...")

    // Block forever
    select {}
}