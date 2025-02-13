package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type header", nil)
		return
	}

	imagePath := getAssetPath(mediaType)
	saveFile, err := os.Create(cfg.getAssetDiskPath(imagePath))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}
	defer saveFile.Close()

	_, err = io.Copy(saveFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error copying image", err)
		return
	}

	videoDB, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to get video from database", err)
		return
	}

	if videoDB.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", nil)
		return
	}

	imageURL := cfg.getAssetURL(imagePath)
	videoDB.ThumbnailURL = &imageURL
	videoDB.UpdatedAt = time.Now()

	err = cfg.db.UpdateVideo(videoDB)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoDB)
}
