# Acuity

---

## About

Acuity is a selfhosted photomanagement service.

## Install

```bash
make docker
make up
```

> [!NOTE]
> Weaviate will take some GB to download its model.

## Features

- find duplicates of images
- find images by text/image
- move/copy images between galleries
- gallery culling (tbd)
- image rating (tbd)
- get image info (tbd)

---

## TODO

- [x] Getting info for one image
- [ ] Paging
- [x] Batch-Delete for selected images
- [x] Count the amount of images in a gallery
- [ ] Copy to gallery, currently only move
- [ ] Add rating system
- [ ] Image carousel
