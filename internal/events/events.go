// Package events moves event image files between a picked directory and one
// event: the import, delete and save-as work behind the 事件 page.
//
// It is the event page's file side. The page itself holds no file system code;
// the shell calls these functions and paints the result. Images are grouped the
// way the application did it: iPhone's live photo arrives as a still and a
// same-named .MOV, every other still stands alone, and a video without its
// still is reported instead of guessed at.
package events

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"transactions/internal/assets"
	"transactions/internal/domain"
	"transactions/internal/repository"
)

// maxMotionBytes bounds a live photo's video, which is read into memory before
// it is stored. A live photo is a few seconds long; anything larger is not one.
const maxMotionBytes = 256 << 20

// AddResult is what one 添加图片 did. Problems are the sentences the page shows
// under the toolbar: a file that could not be used, or a batch that needed a
// companion it did not get.
type AddResult struct {
	Added    int
	Problems []string
}

// picked is one still and the motion part that shares its file stem.
type picked struct {
	stem    string
	still   string
	motion  string
	display []byte
}

// AddImages copies the picked files into the day's event and inserts their
// rows. Files that do not belong to a live photo pair are reported, not stored:
// one video alone would otherwise become a "live photo" with no photo.
func AddImages(ctx context.Context, repo repository.Repository, workspaceDir, date string, paths []string) (AddResult, error) {
	var result AddResult
	date = strings.TrimSpace(date)
	if len(paths) == 0 {
		return result, nil
	}
	if _, err := repo.KeyEvent(ctx, date); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return result, fmt.Errorf("先创建这一天的事件再添加图片")
		}
		return result, err
	}

	groups, unsupported, orphanMotions, err := groupFiles(paths)
	if err != nil {
		return result, err
	}
	if len(unsupported) > 0 {
		result.Problems = append(result.Problems, "不支持的格式："+strings.Join(unsupported, "、"))
	}
	if len(orphanMotions) > 0 {
		result.Problems = append(result.Problems,
			"实况视频要和同名的照片一起选："+strings.Join(orphanMotions, "、"))
	}

	for _, item := range groups {
		problems, err := addOne(ctx, repo, workspaceDir, date, item)
		if err != nil {
			return result, err
		}
		if problems != "" {
			result.Problems = append(result.Problems, problems)
			continue
		}
		result.Added++
	}
	return result, nil
}

// addOne stores one still (and its motion part) and inserts its row. A file
// saved before a later step failed is removed, so a failed image leaves no
// trace on disk.
func addOne(ctx context.Context, repo repository.Repository, workspaceDir, date string, item picked) (string, error) {
	name := filepath.Base(item.still)
	picture, err := assets.Decode(item.display)
	if err != nil {
		return name + "：" + err.Error(), nil
	}
	thumb, err := assets.Thumbnail(picture, assets.ThumbnailWidth)
	if err != nil {
		return name + "：" + err.Error(), nil
	}
	view, err := assets.View(picture, assets.ViewMaxEdge)
	if err != nil {
		return name + "：" + err.Error(), nil
	}
	var motion []byte
	motionExt := ""
	if item.motion != "" {
		motion, err = readLimited(item.motion, maxMotionBytes)
		if err != nil {
			return name + "：" + err.Error(), nil
		}
		motionExt = strings.TrimPrefix(strings.ToLower(filepath.Ext(item.motion)), ".")
	}

	files := assets.Files{
		Display:       item.display,
		DisplayFormat: assets.Sniff(item.display),
		Thumb:         thumb,
		View:          view,
		Motion:        motion,
		MotionExt:     motionExt,
	}
	if files.DisplayFormat == assets.FormatHEIC {
		// HEIC is decoded once, here: the display copy becomes JPEG (92, as the
		// application transcoded) and the original bytes are kept beside
		// it, so a paint never runs the HEVC decoder again and the best copy
		// is never lost.
		display, err := assets.EncodeJPEG(picture, assets.DisplayJPEGQuality)
		if err != nil {
			return name + "：" + err.Error(), nil
		}
		files.Display = display
		files.DisplayFormat = assets.FormatJPEG
		files.Original = item.display
		files.OriginalFormat = assets.FormatHEIC
	}
	id := assets.NewName()
	stored, err := assets.Save(workspaceDir, date, id, files)
	if err != nil {
		return name + "：" + err.Error(), nil
	}
	remove := func() {
		assets.Remove(workspaceDir, stored.FilePath, stored.ThumbPath, stored.OriginalPath, stored.MotionPath, stored.ViewPath)
	}
	if _, err := repo.CreateKeyEventImage(ctx, domain.KeyEventImage{
		ID:           id,
		EventDate:    date,
		FilePath:     stored.FilePath,
		ThumbPath:    stored.ThumbPath,
		OriginalPath: stored.OriginalPath,
		MotionPath:   stored.MotionPath,
	}); err != nil {
		remove()
		return name + "：" + err.Error(), nil
	}
	return "", nil
}

// groupFiles sorts picked paths into live-photo pairs. A still is recognized by
// its bytes, so a renamed photo still imports; a motion part is recognized by
// its extension, as the application did. Files are paired by the lowercased
// file stem, and the first of each kind wins.
func groupFiles(paths []string) (groups []picked, unsupported, orphanMotions []string, err error) {
	type pair struct {
		still   string
		display []byte
		motion  string
	}
	byStem := map[string]*pair{}
	order := make([]string, 0, len(paths))
	at := func(stem string) *pair {
		item, exists := byStem[stem]
		if !exists {
			item = &pair{}
			byStem[stem] = item
			order = append(order, stem)
		}
		return item
	}

	for _, path := range paths {
		info, statErr := os.Stat(path)
		if statErr != nil || info.IsDir() {
			unsupported = append(unsupported, filepath.Base(path))
			continue
		}
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		if motionExtension(ext) {
			item := at(stemOf(path))
			if item.motion == "" {
				item.motion = path
			}
			continue
		}
		data, readErr := readLimited(path, assets.MaxImageBytes)
		if readErr != nil {
			unsupported = append(unsupported, filepath.Base(path)+"（"+readErr.Error()+"）")
			continue
		}
		if assets.Sniff(data) == "" {
			unsupported = append(unsupported, filepath.Base(path))
			continue
		}
		item := at(stemOf(path))
		if item.still == "" {
			item.still = path
			item.display = data
		}
	}

	for _, stem := range order {
		item := byStem[stem]
		if item.still == "" {
			if item.motion != "" {
				orphanMotions = append(orphanMotions, filepath.Base(item.motion))
			}
			continue
		}
		groups = append(groups, picked{
			stem: stem, still: item.still, motion: item.motion, display: item.display,
		})
	}
	return groups, unsupported, orphanMotions, nil
}

// stemOf is the lowercased name without its extension, the key a live photo's
// two files share.
func stemOf(path string) string {
	base := filepath.Base(path)
	return strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
}

// motionExtension reports whether ext names a live photo's video.
func motionExtension(ext string) bool {
	switch ext {
	case "mov", "mp4", "m4v", "webm":
		return true
	}
	return false
}

// readLimited reads a file, refusing one larger than limit.
func readLimited(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("读取图片失败：%w", err)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("文件过大（超过 %d MB）", limit>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取图片失败：%w", err)
	}
	return data, nil
}

// DeleteEvent removes the day's event and its rows in one transaction, then its
// files. The database comes first: a file that could not be removed is a leak,
// while a row without files would be a broken picture in the page.
func DeleteEvent(ctx context.Context, repo repository.Repository, workspaceDir, date string) error {
	images, err := repo.DeleteKeyEvent(ctx, date)
	if err != nil {
		return err
	}
	for _, image := range images {
		assets.Remove(workspaceDir, image.FilePath, image.ThumbPath, image.OriginalPath, image.MotionPath, assets.ViewPath(image.FilePath))
	}
	return nil
}

// DeleteImage removes one image row and its files.
func DeleteImage(ctx context.Context, repo repository.Repository, workspaceDir, imageID string) error {
	image, err := repo.DeleteKeyEventImage(ctx, imageID)
	if err != nil {
		return err
	}
	if image.ID == "" {
		return nil
	}
	assets.Remove(workspaceDir, image.FilePath, image.ThumbPath, image.OriginalPath, image.MotionPath, assets.ViewPath(image.FilePath))
	return nil
}

// SaveImage writes one stored image to the destination the user picked.
func SaveImage(workspaceDir, relativePath, destination string) error {
	return assets.Copy(workspaceDir, relativePath, destination)
}
