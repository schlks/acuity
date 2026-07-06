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

Follow this [guide](https://docs.docker.com/get-started/introduction/get-docker-desktop/) for the installation (you need WSL for windows)

and download the [acuity repo](https://codeberg.org/Shlks/acuity/archive/master.zip) and extract it.

### Step 2

Edit the volume in the [docker-compose](docker-dev.yml) to where
your images are. On Windows you need to replace the drive letter with `/mnt/` followed by the drive letter in lowercase,
for example `C:` becomes `/mnt/c`. Also use `/` instead of `\`.
Since you can create separate galleries for your folders, I
recommend setting the volume to the most root folder you can, or
do something like this:

```yaml
volumes:
 - /your/folder/1:/images/1:rw
 - /your/folder/2:/images/2:rw
```

While you're at it enable CUDA, if you have an Nvidia card:

```yaml
environment:
	ENABLE_CUDA: '1'
```

### Step 3

Open your preferred terminal in the acuity folder and run `docker compose up -d`

> [!NOTE]
> Weaviate will take some GB (~8.5 GB) to download its model.
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
        - ./acuity.db:/app/acuity.db
        - ./config.json:/app/config.json
        - ./your/folder:/data/folder:rw
    environment:
        - WEAVIATE_HOST=weaviate
        - WEAVIATE_PORT=:50050
    restart: on-failure
```

### Linux/MacOS

Install `make` with your package manager, then run:

```bash
make docker
make up
```

### Windows

Run [docker-run.bat](docker-run.bat).

## Features

- find duplicates of images
- find images by text/image
- move/copy images between galleries
- gallery culling (tbd)
- image rating
- get image info

---

## TODO

### Backend

---

### Frontend

- [ ] Image carousel
- [ ] Keybindings
- [x] Search sorting options
- [x] Edit Gallery
- [x] Multiple image selection
- [ ] Drop image in Searchbar
- [x] Delete image/selection
- [x] Image rating
- [x] Settings
