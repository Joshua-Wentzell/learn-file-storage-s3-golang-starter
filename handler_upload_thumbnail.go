package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()
	mediaType := header.Header.Get("Content-Type")
	mediaType = strings.Split(mediaType, "/")[1]
	vidMeta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get video from DB", err)
		return
	}
	if vidMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not the owner of this video", err)
		return
	}
	filePath := filepath.Join(cfg.assetsRoot, videoIDString+"."+mediaType)
	fmt.Println(filePath)
	newFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}
	_, err3 := io.Copy(newFile, file)
	if err3 != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to copy file data", err3)
		return
	}
	newVidURL := fmt.Sprintf("http://localhost:%v/assets/%v.%v", cfg.port, videoID, mediaType)
	vidMeta.ThumbnailURL = &newVidURL
	err4 := cfg.db.UpdateVideo(vidMeta)
	if err4 != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video in DB", err4)
		return
	}
	respondWithJSON(w, http.StatusOK, vidMeta)
}
