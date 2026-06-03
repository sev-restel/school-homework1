package handlers

import (
	crand "crypto/rand"
	"encoding/hex"
	"path"
	"path/filepath"
	"strings"
)

// uniqueDiskName строит уникальное имя файла на диске, чтобы две загрузки
// с одинаковым исходным именем (например, "answer.docx") не перезаписывали
// друг друга. К имени добавляется 6 случайных байт в hex.
//   "answer.docx" -> "answer_3f1a9c20b4d7.docx"
func uniqueDiskName(orig string) string {
	base := filepath.Base(orig)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	b := make([]byte, 6)
	_, _ = crand.Read(b)
	return name + "_" + hex.EncodeToString(b) + ext
}

// normClass приводит название класса к каноничному виду:
// убирает пробелы по краям и переводит в верхний регистр,
// чтобы "10б", "10Б", " 10Б " считались одним классом.
func normClass(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// uploadURLPath — путь, который мы храним в БД и отдаём фронту.
// Всегда через прямой слэш ("uploads/xxx"), чтобы ссылка работала как URL
// и на Windows (где filepath.Join поставил бы бэкслеш).
func uploadURLPath(diskName string) string {
	return path.Join("uploads", diskName)
}
