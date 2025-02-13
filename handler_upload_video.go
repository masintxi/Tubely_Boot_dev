package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const uploadLimit = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)

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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find video", err)
		return
	}

	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "User missmatch", nil)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	r.ParseMultipartForm(uploadLimit)

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to validate video", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type for a video", nil)
		return
	}

	tempVideo, err := os.CreateTemp("", "tubely-upload.pm4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to create temporary video", err)
		return
	}
	defer os.Remove(tempVideo.Name())
	defer tempVideo.Close()

	_, err = io.Copy(tempVideo, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error copying video", err)
		return
	}

	_, err = tempVideo.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to reset file pointer", err)
		return
	}

	procFilePath, err := processVideoForFastStart(tempVideo.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error preprocessing video", err)
		return
	}
	defer os.Remove(procFilePath)

	procVideo, err := os.Open(procFilePath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to open procesed video", err)
		return
	}
	defer procVideo.Close()

	videoRatio, err := getVideoAspectRatio(procVideo.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to calculate video ratio", err)
		return
	}

	videoKey := getAssetPath(mediaType)
	videoKey = filepath.Join(videoRatio, videoKey)

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &videoKey,
		Body:        procVideo,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading video", err)
		return
	}

	videoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, videoKey)
	video.VideoURL = &videoURL
	video.UpdatedAt = time.Now()

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)

}
