# SimpleDevicePlugin
This is a very simple k8s device plugin that allows forwarding of devices. It does not attempt to make sure the device is actually on a node.

## Usage
Pass 1 argument for each set of device paths you want to expose as a resource
```
'{"name": "gpu", "paths": ["/dev/dri/renderD128", "/dev/dri/card1"]}' '{"name": "zwave", "paths": ["/dev/ttyACM0"]}'
```

These will then be exposed on your cluster in the form
```
threax.com/gpu
threax.com/zwave
```

You can use these from a manifest by writing the resource names you want under the limits. You can use any subset of what you defined above.
```
    resources:
      limits:
        threax.com/gpu: 1
        threax.com/zwave: 1
```
Make sure to schedule the pod to a node that actually has these devices, since this plugin does not check.

## Based On

https://oneuptime.com/blog/post/2026-02-09-device-plugin-custom-hardware-kubernetes/view