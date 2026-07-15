<p align="center">
 <img src="assets/logo.svg" alt="Acuity Logo" width="300"/>
</p>

> acuity  
> /ə-kyoo͞′ĭ-tē/  
> noun  
>
> 1. a quick and penetrating intelligence
> 2. sharpness of vision; the visual ability to resolve fine detail

Acuity is a self-hosted photo management service.
It aims to help you find and organize your photos with precision and
speed. Designed as a high-performance image management and labeling
tool, it allows you to maintain complete control over your library
while running entirely on your own infrastructure.

---

## Install

### Step 1

(If you know what docker and git is and how to install skip to step 2)

Follow this [guide](https://docs.docker.com/get-started/introduction/get-docker-desktop/) for the installation (you need WSL for windows).

Download the [acuity repo](https://codeberg.org/Shlks/acuity/archive/master.zip) and extract it.

### Step 2

Edit the volume in the [docker-compose](docker-dev.yml) to where
your images are. On Windows you need to use `/` instead of `\`.
Since you can create separate galleries for your folders, I
recommend setting the volume to the most root folder you can, or
do something like this:

```yaml
volumes:
  - /your/folder/1:/images/1:rw
  - /your/folder/2:/images/2:rw
```

While you're at it enable [CUDA](https://docs.docker.com/compose/how-tos/gpu-support/), if you have an Nvidia card:

```yaml
environment:
  ENABLE_CUDA: '1'
```

### Step 3

Open your preferred terminal in the acuity folder and run `docker compose up -d`

> [!NOTE]
> Weaviate will take ~8 GB to download its model. If you want a smaller (or larger) model head over to
> [the Weaviate docker install guide](https://docs.weaviate.io/deploy/installation-guides/docker-installation) and choose a model for your needs.
>
> You can also install `Docker Desktop` on Linux if you want to.

## Developing

Replace the acuity container in the [docker-compose](docker-dev.yml) with

```yml
acuity:
  # image: codeberg.org/shlks/acuity:latest
  # pull_policy: always
  build: # needed for building the container before deploying
    context: .
    pull: true
  container_name: acuity
  ports:
    - "3000:3000"
  volumes:
    - acuity_data:/app/data
    - ./your/folder:/data/folder:rw
  environment:
    - WEAVIATE_HOST=weaviate
    - WEAVIATE_PORT=:50050
  restart: on-failure
```

### Linux/MacOS

Run [run.sh](run.sh).

### Windows

Run [docker-run.bat](docker-run.bat).

## Features

- find duplicates of images
- find images by text/image
- move/copy images between galleries
- gallery culling
- image rating
- get image info

---

## TODO

### Backend

---

### Frontend

- [ ] Polishing
- [ ] Refactoring to have 0 AI in this project

---

### AI was used to assist writing code
