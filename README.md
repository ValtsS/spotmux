# SpotMux Docker Guide

This guide explains how to build and run SpotMux using Docker.

## Building the Image

To build the Docker image locally, run the following command from the root of the project:

```bash
docker build -t spotmux .
```

## Running the Container

Once the image is built, you can run it using:

```bash
docker run -d \
  -p 7300:7300 \
  --name spotmux \
  spotmux
```

### Configuration

SpotMux expects a `config.json` file. You can mount your local configuration into the container:

```bash
docker run -d \
  -p 7300:7300 \
  -v $(pwd)/config.json:/home/spotmux/config.json \
  --name spotmux \
  spotmux
```

### Security

The container is configured to run as a non-root user (`spotmux`) for improved security. It also includes `ca-certificates` for secure connections and `tzdata` for timezone support.
