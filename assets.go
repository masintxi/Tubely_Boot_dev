package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	key := make([]byte, 32)
	rand.Read(key)
	fileName := base64.RawURLEncoding.EncodeToString(key)

	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", fileName, ext)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var result bytes.Buffer
	cmd.Stdout = &result

	err := cmd.Run()
	if err != nil {
		log.Println("error in ffprobe run:", err)
		return "", err
	}

	var data struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}

	err = json.Unmarshal(result.Bytes(), &data)
	if err != nil {
		log.Println("error unmarshalling data:", err)
		return "", err
	}

	if len(data.Streams) == 0 {
		log.Println("no video stream found")
		return "", fmt.Errorf("no video stream found")
	}

	ratio := float64(data.Streams[0].Width) / float64(data.Streams[0].Height)

	if math.Abs(ratio-float64(16)/float64(9)) < 0.5 {
		//fmt.Println("landscape")
		return "landscape", nil
	}
	if math.Abs(ratio-float64(9)/float64(16)) < 0.5 {
		//fmt.Println("portrait")
		return "portrait", nil
	}
	return "other", nil
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath,
		"-c", "copy", "-movflags",
		"faststart", "-f", "mp4", outputFilePath)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("error in processing video: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
		return "", err
	}

	return outputFilePath, nil
}

// func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
// 	presClient := s3.NewPresignClient(s3Client)
// 	s3Object := s3.GetObjectInput{
// 		Bucket: &bucket,
// 		Key:    &key,
// 	}

// 	presURL, err := presClient.PresignGetObject(context.TODO(), &s3Object, s3.WithPresignExpires(expireTime))
// 	if err != nil {
// 		fmt.Println("error in presinggetobject")
// 		return "", err
// 	}

// 	return presURL.URL, nil
// }

// func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
// 	if video.VideoURL == nil {
// 		return video, nil
// 	}

// 	params := strings.Split(*video.VideoURL, ",")
// 	if len(params) != 2 {
// 		return database.Video{}, nil
// 	}
// 	bucket := params[0]
// 	key := params[1]
// 	newURL, err := generatePresignedURL(cfg.s3Client, bucket, key, 5*time.Minute)
// 	if err != nil {
// 		return database.Video{}, err
// 	}
// 	video.VideoURL = &newURL

// 	return video, nil
// }
