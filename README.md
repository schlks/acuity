<p align="center">
	<img src="assets/logo.svg" alt="Acuity Logo" width="300"/>
</p>

> acuity  
> /ə-kyoo͞′ĭ-tē/  
> noun  
>
> 1. a quick and penetrating intelligence
> 2. sharpness of vision; the visual ability to resolve fine detail

Acuity is a self-hosted photomanagement service.
It aims to help you find and organize your photos with precision and
speed. Designed as a high-performance image management and labeling
tool, it allows you to maintain complete control over your library
while running entirely on your own infrastructure.

---

## Install

### Step 1

Edit the volume in the [docker-compose](docker-compose.yml) to where
your images are.
Change `:ro` to `:rw` if you want to move/copy/delete files.
Since you can create separate galleries for your folders, I
recommend setting the volume to the most root folder you can, or
do something like this:

```yaml
volumes:
 - /your/folder/1:/images/1:ro
 - /your/folder/2:/images/2:ro
```

While you're at it enable CUDA, if you have a Nvidia card:

```yaml
environment:
	ENABLE_CUDA: '1'
```

### Step 2 (Linux)

Install `make` and `docker`, with your package manager, then run

```bash
make docker
make up
```

### Step 2 (Windows)

Install [Docker Desktop](https://www.docker.com/products/docker-desktop/)
and run [docker-run.bat](docker-run.bat).

### Step 2 (MacOS)

Install [Docker Desktop](https://www.docker.com/products/docker-desktop/),
then run

```bash
make docker
make up
```

> [!NOTE]
> Weaviate will take some GB to download its model.
>
> You can also install `Docker Desktop` on Linux if you want to.

## Features

- find duplicates of images
- find images by text/image
- move/copy images between galleries
- gallery culling (tbd)
- image rating
- get image info

---

## TODO

- [x] Getting info for one image
- [x] Paging
- [x] Batch-Delete for selected images
- [x] Count the amount of images in a gallery
- [x] Copy to gallery, currently only move
- [x] Add rating system
- [ ] Image carousel
- [x] Sort Search/Gallery by Date/Extension
- [ ] Keybindings
- [x] Settings
- [ ] In general the frontend
