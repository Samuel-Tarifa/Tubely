package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/media"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30
	defer r.Body.Close()
	http.MaxBytesReader(w, r.Body, maxMemory)
	videoIDString := r.PathValue("videoID")
	videoUUID, err := uuid.Parse(videoIDString)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "uuid invalid", err)
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

	video, err := cfg.db.GetVideo(videoUUID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "video not found", err)
			return
		} else {
			respondWithError(w, http.StatusInternalServerError, "error getting video", err)
			return
		}
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "not owner", err)
		return
	}

	videoFile, videoFileHeader, err := r.FormFile("video")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "error parsing video", err)
		return
	}

	defer videoFile.Close()

	mediaType, _, err := mime.ParseMediaType(videoFileHeader.Header.Get("Content-Type"))

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error parsing mediaType", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "mediaType not allowed", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating temp file", err)
		return
	}

	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, videoFile)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error copying file", err)
		return
	}

	fastFilepath, err := media.ProcessVideoForFastStart(tempFile.Name())

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error processing video for fast start", err)
		return
	}

	fastFile, err := os.Open(fastFilepath)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error oppening fastFile", err)
		return
	}

	defer os.Remove(fastFile.Name())
	defer fastFile.Close()

	aspectRatio, err := media.GetVideoAspectRatio(fastFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting aspect ratio", err)
		return
	}

	prefix := ""

	switch aspectRatio {
	case "16:9":
		prefix = "landscape/"
	case "9:16":
		prefix = "portrait/"
	case "other":
		prefix = "other/"
	}

	v := make([]byte, 32)
	rand.Read(v)
	fileKey := base64.RawURLEncoding.EncodeToString(v) + ".mp4"
	finalKey := prefix + fileKey

	params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &finalKey,
		Body:        fastFile,
		ContentType: &mediaType,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &params)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading video to aws", err)
		return
	}

	newURL := fmt.Sprintf("https://%s/%s", cfg.s3CfDistribution, finalKey)

	video.VideoURL = &newURL

	err=cfg.db.UpdateVideo(video)

	if err!=nil{
		respondWithError(w,http.StatusInternalServerError,"error updating video",err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
