# Acuity

---

## About

Acuity is a selfhosted photomanagement service.

## Install (Linux)

Edit the volume in the [docker-compose](docker-compose.yml) to where
your images are.
Change `:ro` to `:rw` if you want to move/copy/delete files.
Since you can create separate galleries for your folders, I
recommend setting the Volume to the most root folder you can, or
do something like this:

```yaml
volumes:
 - ./config.yaml:/app/config.yaml
 - /your/folder/1:/images/1:ro
 - /your/folder/2:/images/2:ro
```

After editing the compose file, you can run:

```bash
make docker
make up
```

> [!NOTE]
> Weaviate will take some GB to download its model.

## Features

- find duplicates of images
- find images by text/image
- move/copy(tbd) images between galleries
- gallery culling (tbd)
- image rating (tbd)
- get image info

---

## TODO

- [x] Getting info for one image
- [ ] Paging
- [x] Batch-Delete for selected images
- [x] Count the amount of images in a gallery
- [ ] Copy to gallery, currently only move
- [ ] Add rating system
- [ ] Image carousel
- [ ] Sort Search/Gallery by Date/Extension
- [ ] Keybindings
