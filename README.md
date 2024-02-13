# github.com/refcall/next-image-s3-vips

This is a `next/image` kind-of compliant optimizer.

## Getting started

### Backend

#### Environment

```
BACKEND_S3=localhost:9000
BACKEND_S3_SECURE=false
BACKEND_STORAGE_PATH=/tmp
```

### Frontend

#### `next.config.js`

```typescript
const nextConfig = {
  images: {
    loader: 'custom',
    loaderFile: './next-image-s3-vips.js',
  }
}
```

#### `next-image-s3-vips.js`

```typescript
export default function myImageLoader({ src, width, quality }) {
  if (process.env.NEXT_PUBLIC_IMAGE_OPTIMIZER === undefined || src.startsWith('http') || src.startsWith('/')) {
    return src
  }
  return `${process.env.NEXT_PUBLIC_IMAGE_OPTIMIZER}/${src}?w=${width}&q=${quality || 75}`
}
```

#### Environment
```
NEXT_PUBLIC_IMAGE_OPTIMIZER=http://localhost:4050
```


## Comparison

### Pros

- less buggy-ish
  - in heavy traffic condition, `next/image` sends a wrong image instead of the requested one
- faster
  - it connects directly to s3
  - streams the image if possible between components (s3 and transcoder)
  - uses the `vips` library which is made for speed

### Cons

- not made for HTTP resources
- files not as optimized (compressed) as `next/image`
- must spin-up another service on the infrastructure
- not as configurable
- need a proxy for CORS
- fallback on the real image as this tool is not made to read external images