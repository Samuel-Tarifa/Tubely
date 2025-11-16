package main

import (
	"database/sql"
	"errors"
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

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, 500, "error parsing multipartForm", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to parse form file", err)
		return
	}

	defer file.Close()

	mediaType := header.Header.Get("Content-Type")

	metadata, err := cfg.db.GetVideo(videoID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "no video found", err)
			return
		}
		respondWithError(w, http.StatusInternalServerError, "database error", err)
		return
	}

	if metadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "unauthorized", err)
		return
	}

	extension:=strings.Split(mediaType,"/")[1]

	path:=filepath.Join(cfg.assetsRoot,videoIDString+"."+extension)

	createdFile,err:=os.Create(path)

	if err!=nil{
		respondWithError(w,http.StatusInternalServerError,"error creating file",err)
		return
	}

	_,err=io.Copy(createdFile,file)

	if err!=nil{
		respondWithError(w,http.StatusInternalServerError,"error copying file",err)
		return
	}

	newThumbnailUrl := fmt.Sprintf("http://localhost:8091/assets/%s.%s",videoID,extension)

	metadata.ThumbnailURL = &newThumbnailUrl

	err = cfg.db.UpdateVideo(metadata)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video url", err)
		return
	}

	respondWithJSON(w, http.StatusOK, metadata)
}
