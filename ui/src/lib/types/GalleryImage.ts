export interface GalleryImage {
    id: number;
    gallery_id: number;
    filepath: string;
    file_hash?: string;
    blurhash: string;
    width?: number;
    height?: number;
    rating: number;
    flag: number;
    tags?: string;
    extension: string;
    date: string;
    size: number;
    resolution: number;
    taken: string;
    aspect_ratio: number;
    camera_make?: string;
    lens_make?: string;
    focal_length?: string;
    aperture?: string;
    shutter_speed?: string;
    iso?: string;
    flash?: boolean;
    distance?: number;
}