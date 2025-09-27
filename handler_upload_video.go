package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)
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
	vidMeta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get video from DB", err)
		return
	}
	if vidMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not the owner of this video", err)
		return
	}
	formFile, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to read video data", err)
		return
	}
	defer formFile.Close()
	mediaType := header.Header.Get("Content-Type")
	mediaType, _, _ = mime.ParseMediaType(mediaType)
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file format", err)
		return
	}
	fileParts := strings.Split(mediaType, "/")
	fileExt := fileParts[len(fileParts)-1]
	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create temp file", err)
		return
	}
	defer os.Remove("tubely-upload.mp4")
	defer tempFile.Close()
	if _, err := io.Copy(tempFile, formFile); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to copy video data to temp file", err)
		return
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to move file pointer back to start", err)
		return
	}
	byteSlice := make([]byte, 32)
	rand.Read(byteSlice)
	randFileName := base64.RawURLEncoding.EncodeToString(byteSlice) + "." + fileExt
	putObjectInput := &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &randFileName,
		Body:        tempFile,
		ContentType: &mediaType,
	}
	cfg.s3Client.PutObject(context.TODO(), putObjectInput)
	newVidURL := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, randFileName)
	vidMeta.VideoURL = &newVidURL
	if err := cfg.db.UpdateVideo(vidMeta); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video in DB", err)
		return
	}
}
