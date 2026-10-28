# Build from Source on Ubuntu

I prefer the Docker container for Podgrab.
Many community members prefer native installation on their Linux servers because they are not comfortable with Docker.

This guide targets Ubuntu.
For other Linux distributions, familiarity with command-line tools is likely sufficient to adapt these steps.

## Install Go

This repository requires Go 1.27.3 or later to build Podgrab from source.

If you already have Go installed on your machine, you can skip to the next step.

Get precise Go installation process at the official link here -
https://golang.org/doc/install

These steps work only with a correct Go installation and configuration.

## Install the Dependencies

```bash
 sudo apt-get install -y git ca-certificates ufw gcc
```

## Clone from Git

```bash
git clone --depth 1 https://github.com/akhilrex/podgrab
```

## Build and Copy dependencies

```bash
cd podgrab
mkdir -p ./dist
cp .env ./dist
go build -o ./dist/podgrab ./main.go
```

## Create final destination and copy executable

```bash
sudo mkdir -p /usr/local/bin/podgrab
mv -v dist/* /usr/local/bin/podgrab
mv -v dist/.* /usr/local/bin/podgrab
```

At this point, the installation is complete in principle.
Adjust `.env` in `/usr/local/bin/podgrab` for your configuration.
Run this command:

```bash
cd /usr/local/bin/podgrab && ./podgrab
```

Point your browser to http://localhost:8080 (on the same machine) or
`http://server-ip:8080` from other machines. Replace `server-ip` with the server address.

If you use ufw or another firewall, you possibly need an exception
for this port on that.

## Setup as service (Optional)

If you want to run Podgrab in the background as a service or auto-start whenever
the server starts, follow the next steps.

Create new file named `podgrab.service` at `/etc/systemd/system` and add the
following content. You will have to modify the content accordingly if you
changed the installation path in the previous steps.

```unit
[Unit]
Description=Podgrab

[Service]
ExecStart=/usr/local/bin/podgrab/podgrab
WorkingDirectory=/usr/local/bin/podgrab/
[Install]
WantedBy=multi-user.target
```

Run the following commands

```bash
sudo systemctl daemon-reload
sudo systemctl enable podgrab.service
sudo systemctl start podgrab.service
```

Run the following command to check the service status.

```bash
sudo systemctl status podgrab.service
```

# Update Podgrab

To update an installed copy of Podgrab to the latest version, repeat the clone, build, and copy steps.
Docker also simplifies this process.

Stop the active service if you use it

```bash
sudo systemctl stop podgrab.service
```

## Clone from Git

```bash
git clone --depth 1 https://github.com/akhilrex/podgrab
```

## Build and Copy dependencies

```bash
cd podgrab
mkdir -p ./dist
cp .env ./dist
go build -o ./dist/podgrab ./main.go
```

## Create final destination and copy executable

```bash
sudo mkdir -p /usr/local/bin/podgrab
mv -v dist/* /usr/local/bin/podgrab
```

Restart the service if you use it

```bash
sudo systemctl start podgrab.service
```
