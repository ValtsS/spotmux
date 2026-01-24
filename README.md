# SpotMux Docker Guide

SpotMux is a lightweight DX (Distance X) Spot Aggregator and Telnet Server for amateur radio. It connects to multiple upstream DX clusters, aggregates their spot feeds, removes duplicates, and serves the unified stream to downstream clients. 

Key features include:
- **Upstream Aggregation**: Connects to multiple DX clusters simultaneously.
- **Deduplication & Caching**: Filters out redundant spots and keeps a recent cache for new connections.
- **Telnet Compatibility**: Provides a standard telnet interface for loggers and other DX clients.
- **Microservice Ready**: Designed to run efficiently in Docker as a non-privileged service.

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
