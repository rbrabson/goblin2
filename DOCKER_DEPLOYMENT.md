# Docker Setup and Deployment Guide

This guide covers building, deploying, and running the Goblin Discord bot using Docker.

## 📦 Building Docker Images

### Local Build

Build the image locally for testing:

```bash
# Build with default tag
docker docker build --no-cache --platform=linux/amd64 --tag goblin .

# Build with specific tag
docker docker build --no-cache --platform=linux/amd64 --tag goblin v1.0.0 .

# Build with multiple tags
docker docker build --no-cache --platform=linux/amd64  --tag goblin:latest --tag goblin:v1.0.0 .
```

### Build Arguments (if needed)

```bash
# Example with build arguments
docker docker build --no-cache --platform=linux/amd64 --build-arg GO_VERSION=1.26 --tag goblin .
```

## 🏷️ Tagging Images

### Tag for Docker Hub

```bash
# Tag for a Docker registry
docker tag goblin:latest registry-name/goblin:latest
docker tag goblin:latest registry-name/goblin:v1.0.0
```

## 🚀 Pushing Images

### Push to Docker Registry

```bash
# Login to your Docker registry
docker login

# Push specific version
docker push registry-name/goblin:v1.0.0

# Push latest
docker push registry-name/goblin:latest

# Push all tags
docker push registry-name/goblin --all-tags
```

## 📥 Pulling Images

### Pull from Docker Registry

```bash
# Pull latest version
docker pull registry-name/goblin:latest

# Pull specific version
docker pull registry-name/goblin:v1.0.0
```

## 🔧 Docker Compose Deployment

### Option 1: Using Pre-built Image with Environment Variables

1. **Create or use the `docker-compose.yaml` file:**

    ```yaml
    services:
      goblin:
        container_name: "goblin"
        build:
          context: .
          dockerfile: ./Dockerfile
        environment:
          GOBLIN_CONFIG_PATH: /yaml
        entrypoint: /goblin
        depends_on:
          - mongodb
    
      mongodb:
        container_name: "goblin_mongo"
        image: mongo:latest
        environment:
          GOBLIN_CONFIG_PATH: /yaml
        ports:
          - "27017:27017"
        volumes:
          - mongodb_data_container:/data/db
    
    volumes:
      mongodb_data_container:
        driver: local
    ```

### Option 2: Using .env File (Recommended)

1. **Create a `.env` file:**

    ```bash
    nano .env
    ```

2. **Edit `.env` with your values:**

    ```env
    # Discord Bot Configuration
    GOBLIN_CONFIG_PATH: /yaml
    ```

3. **Update Docker Compose to use .env:**

    ```yaml
    services:
      goblin:
        container_name: "goblin"
        build:
          context: .
          dockerfile: ./Dockerfile
        env_file: ./.env
        entrypoint: /goblin
        depends_on:
          - mongodb
    
      mongodb:
        container_name: "goblin_mongo"
        image: mongo:latest
        env_file: ./.env
        ports:
          - "27017:27017"
        volumes:
          - mongodb_data_container:/data/db
    
    volumes:
      mongodb_data_container:
        driver: local
    ```

## 🚀 Running the Application

### Setup Steps

1. **Edit configuration:**
    - Update `docker-compose.yaml` with your image name
    - Edit `.env` with your actual values (if using .env option)
    - Or edit environment variables directly in `docker-compose.yaml`

2. **Start services:**

   ```bash
   # Start in background
   docker-compose up -d
   
   # Start with logs visible
   docker-compose up
   
   # Build and start (if using build option)
   docker-compose up --build -d
   ```

### Management Commands

```bash
# View logs
docker-compose logs -f goblin
docker-compose logs -f mongodb

# Stop services
docker-compose stop

# Stop and remove containers
docker-compose down

# Stop and remove containers + volumes
docker-compose down -v

# Restart specific service
docker-compose restart goblin

# Update and restart (pull new image)
docker-compose pull
docker-compose up -d

# View running services
docker-compose ps

# Execute commands in running container
docker-compose exec goblin /bin/sh
```

## 🔒 Security Best Practices

1. **Use .env files for sensitive data**
2. **Never commit .env files to version control**
3. **Use specific image tags instead of 'latest' in production**
4. **Regularly update base images for security patches**
5. **Use secrets management for production deployments**

## 🐛 Troubleshooting

### Common Issues

**Container exits immediately:**

```bash
# Check logs
docker-compose logs goblin

# Check if environment variables are set
docker-compose exec goblin env
```

**MongoDB connection issues:**

```bash
# Check MongoDB logs
docker-compose logs mongodb

# Test MongoDB connection
docker-compose exec mongodb mongosh
```

**Image pull failures:**

```bash
# Login to registry
docker login

# Check image exists
docker pull username/goblin:latest
```

### Debug Mode

Run with debug logging:

```bash
# Set LOG_LEVEL=debug in .env or docker-compose.yaml
LOG_LEVEL=debug docker-compose up
```

