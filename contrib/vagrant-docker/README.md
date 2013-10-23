# Vagrant-docker

This is a placeholder for the official vagrant-docker, a plugin for Vagrant (http://vagrantup.com) which exposes Docker as a provider.

## Setting up Vagrant-docker with the Remote API

The initial Docker upstart script will not work because it runs on `127.0.0.1`, which is not accessible to the host machine. Instead, we need to change the script to connect to `0.0.0.0`. To do this, modify `/etc/init/docker.conf` to look like this:

```
description     "Docker daemon"

start on filesystem and started lxc-net
stop on runlevel [!2345]

respawn

script
    /usr/bin/docker -d -H=tcp://0.0.0.0:4243/
end script
```

Once that's done, you need to set up a SSH tunnel between your host machine and the vagrant machine that's running Docker. This can be done by running the following command in a host terminal:

```
ssh -L 4243:localhost:4243 -p 2222 vagrant@localhost
```

(The first 4243 is what your host can connect to, the second 4243 is what port Docker is running on in the vagrant machine, and the 2222 is the port Vagrant is providing for SSH. If VirtualBox is the VM you're using, you can see what value "2222" should be by going to: Network > Adapter 1 > Advanced > Port Forwarding in the VirtualBox GUI.)

Note that because the port has been changed, to run docker commands from within the command line you must run them like this:

```
sudo docker -H 0.0.0.0:4243 < commands for docker >
```
