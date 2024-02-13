package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/elnormous/contenttype"
	"github.com/minio/minio-go/v7"
)

var VersionRelease = "unknown"
var VersionBranch = "unknown"
var VersionCommit = "unknown"

var webp = contenttype.NewMediaType("image/webp")
var png = contenttype.NewMediaType("image/png")
var jpeg = contenttype.NewMediaType("image/jpeg")

func main() {
	log.Println("Starting github.com/refcall/next-image-s3-vips", VersionRelease, VersionBranch, VersionCommit)

	vips.Startup(&vips.Config{})
	defer vips.Shutdown()

	availableMediaTypes := []contenttype.MediaType{
		webp, png, jpeg,
	}

	minioClient, err := minio.New(os.Getenv("BACKEND_S3"), &minio.Options{
		Secure: os.Getenv("BACKEND_S3_SECURE") == "true",
	})
	if err != nil {
		log.Fatalln(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, fmt.Sprintf(`github.com/refcall/next-image-s3-vips

Release: %s
Branch: %s
Commit: %s`, VersionRelease, VersionBranch, VersionCommit))
	})
	mux.HandleFunc("GET /{bucket}/{path...}", func(wrt http.ResponseWriter, req *http.Request) {
		bucket := req.PathValue("bucket")
		path := req.PathValue("path")
		log.Println("request", bucket, path)

		if bucket == "" || path == "" {
			wrt.WriteHeader(http.StatusBadRequest)
			io.WriteString(wrt, "Path must comply to the format `/{bucket}/{path...}`")
			return
		}

		accepted, _, acceptError := contenttype.GetAcceptableMediaType(req, availableMediaTypes)
		if acceptError != nil {
			wrt.WriteHeader(http.StatusBadRequest)
			io.WriteString(wrt, "Cannot accept any type provided by the header `Accept`")
			return
		}

		w := 0
		if req.URL.Query().Get("w") != "" {
			w, err = strconv.Atoi(req.URL.Query().Get("w"))
			if err != nil {
				wrt.WriteHeader(http.StatusBadRequest)
				io.WriteString(wrt, "Query param `w` (width: optional) must be an integer")
				return
			}
		}

		q := 80
		if req.URL.Query().Get("q") != "" {
			q, err = strconv.Atoi(req.URL.Query().Get("q"))
			if err != nil {
				wrt.WriteHeader(http.StatusBadRequest)
				io.WriteString(wrt, "Query param `q` (quality: optional) must be an integer")
				return
			}
		}

		if f, err := getImage(bucket, path, w, q, accepted); err == nil {
			wrt.Header().Add("Cache-Control", "public, max-age=31536000")
			wrt.Header().Add("X-Cache", "HIT")
			io.Copy(wrt, f)
			return
		}

		obj, err := minioClient.GetObject(
			req.Context(),
			bucket,
			path,
			minio.GetObjectOptions{},
		)
		if err != nil {
			wrt.WriteHeader(http.StatusNotFound)
			io.WriteString(wrt, "File cannot be found on the bucket")
		}

		ref, err := vips.NewImageFromReader(obj)
		if err != nil {
			wrt.WriteHeader(http.StatusInternalServerError)
			io.WriteString(wrt, "Cannot read the image")
			return
		}

		if w != 0 {
			err = ref.Resize(float64(w)/float64(ref.Width()), vips.KernelAuto)
			if err != nil {
				wrt.WriteHeader(http.StatusInternalServerError)
				io.WriteString(wrt, "Cannot resize the image")
				return
			}
		}

		if accepted.Equal(webp) {
			ep := vips.NewWebpExportParams()
			ep.Quality = q

			bts, _, err := ref.ExportWebp(ep)
			if err != nil {
				wrt.WriteHeader(http.StatusInternalServerError)
				io.WriteString(wrt, "Cannot export the image")
				return
			}

			go storeImage(bucket, path, w, q, accepted, bts)
			wrt.Header().Add("Cache-Control", "public, max-age=31536000")
			wrt.Header().Add("X-Cache", "MISS")
			wrt.Write(bts)
		}
	})
	s := &http.Server{
		Addr:    ":4050",
		Handler: mux,
	}
	log.Println("Starting on port 4050")
	log.Fatal(s.ListenAndServe())
}

func getImagePath(bucket string, path string, w int, q int, mtype contenttype.MediaType) (string, string) {
	dir := filepath.Join(os.Getenv("BACKEND_STORAGE_PATH"), bucket, filepath.Dir(path))
	m, _ := mime.ExtensionsByType(mtype.String())
	full := filepath.Join(dir, filepath.Base(path)+"_w"+strconv.Itoa(w)+"_q"+strconv.Itoa(q)+m[0])
	return dir, full
}

func getImage(bucket string, path string, w int, q int, mtype contenttype.MediaType) (*os.File, error) {
	_, full := getImagePath(bucket, path, w, q, mtype)
	return os.Open(full)
}

func storeImage(bucket string, path string, w int, q int, mtype contenttype.MediaType, data []byte) {
	dir, full := getImagePath(bucket, path, w, q, mtype)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		log.Println(err)
	}

	if err := os.WriteFile(full, data, os.ModePerm); err != nil {
		log.Println(err)
	}
}
