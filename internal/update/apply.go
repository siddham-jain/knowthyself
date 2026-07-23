package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// maxArchive caps a release download so a wrong URL can't exhaust disk.
const maxArchive = 200 << 20

// Apply downloads the release, verifies it against the published checksums, and
// replaces the binary at dest. The staged file is written beside dest so the final
// swap is an atomic same-filesystem rename.
func Apply(ctx context.Context, rel Release, dest string) error {
	destDir := filepath.Dir(dest)
	if err := checkWritable(destDir); err != nil {
		return err
	}

	staging, err := os.MkdirTemp(destDir, ".knowthyself-update-")
	if err != nil {
		return fmt.Errorf("could not create a staging directory in %s: %w", destDir, err)
	}
	defer os.RemoveAll(staging)

	archive := filepath.Join(staging, rel.AssetName())
	sum, err := download(ctx, rel.assetURL(), archive)
	if err != nil {
		return err
	}
	if err := verify(ctx, rel, sum); err != nil {
		return err
	}

	extracted := filepath.Join(staging, exeName())
	if err := extractBinary(archive, extracted); err != nil {
		return err
	}
	if err := os.Chmod(extracted, modeOf(dest)); err != nil {
		return fmt.Errorf("could not make the new binary executable: %w", err)
	}
	return swap(extracted, dest)
}

func exeName() string {
	if runtime.GOOS == "windows" {
		return binName + ".exe"
	}
	return binName
}

// checkWritable fails before the download rather than after it, so a permission
// problem costs the user a message instead of a 10 MB transfer.
func checkWritable(dir string) error {
	probe, err := os.CreateTemp(dir, ".knowthyself-write-check-")
	if err != nil {
		return fmt.Errorf("cannot write to %s — re-run with elevated permissions, or reinstall with the install script", dir)
	}
	name := probe.Name()
	probe.Close()
	os.Remove(name)
	return nil
}

func modeOf(path string) os.FileMode {
	if fi, err := os.Stat(path); err == nil {
		return fi.Mode().Perm()
	}
	return 0o755
}

// download streams url to path and returns its SHA-256.
func download(ctx context.Context, url, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not download %s: %w", filepath.Base(url), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("could not download %s — github returned %s", filepath.Base(url), resp.Status)
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxArchive))
	if err != nil {
		return "", fmt.Errorf("download of %s was interrupted: %w", filepath.Base(url), err)
	}
	if n == maxArchive {
		return "", fmt.Errorf("%s is larger than the %d MB limit — refusing it", filepath.Base(url), maxArchive>>20)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// verify checks the archive digest against the release's signed checksums file. A
// mismatch aborts the update — a corrupted or tampered archive is never installed.
func verify(ctx context.Context, rel Release, got string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rel.checksumURL(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not fetch checksums.txt: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("could not fetch checksums.txt — github returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	want := ""
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == rel.AssetName() {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("release %s publishes no checksum for %s", rel.Tag, rel.AssetName())
	}
	if !strings.EqualFold(want, got) {
		return fmt.Errorf("checksum mismatch for %s — refusing to install (expected %s, got %s)", rel.AssetName(), want, got)
	}
	return nil
}

// extractBinary pulls just the knowthyself executable out of the release archive.
func extractBinary(archive, out string) error {
	if strings.HasSuffix(archive, ".zip") {
		return extractZip(archive, out)
	}
	return extractTarGz(archive, out)
}

func extractTarGz(archive, out string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("release archive is not valid gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("could not read the release archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != exeName() {
			continue
		}
		return writeFile(out, io.LimitReader(tr, maxArchive))
	}
	return fmt.Errorf("release archive did not contain %s", exeName())
}

func extractZip(archive, out string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("release archive is not a valid zip: %w", err)
	}
	defer zr.Close()

	for _, entry := range zr.File {
		if entry.FileInfo().IsDir() || filepath.Base(entry.Name) != exeName() {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		return writeFile(out, io.LimitReader(rc, maxArchive))
	}
	return fmt.Errorf("release archive did not contain %s", exeName())
}

func writeFile(path string, r io.Reader) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return err
	}
	return f.Close()
}

// swap moves the staged binary over the running one. Unix replaces the directory
// entry while the old inode stays alive for the running process; Windows cannot
// overwrite a running image, so the old file is moved aside first and restored if
// the move fails.
func swap(staged, dest string) error {
	if runtime.GOOS != "windows" {
		if err := os.Rename(staged, dest); err != nil {
			return fmt.Errorf("could not replace %s: %w", dest, err)
		}
		return nil
	}

	backup := dest + ".old"
	os.Remove(backup)
	if err := os.Rename(dest, backup); err != nil {
		return fmt.Errorf("could not move the current binary aside: %w", err)
	}
	if err := os.Rename(staged, dest); err != nil {
		if restore := os.Rename(backup, dest); restore != nil {
			return fmt.Errorf("could not install the new binary (%w) and the old one is left at %s", err, backup)
		}
		return fmt.Errorf("could not install the new binary: %w", err)
	}
	// The running image keeps the backup locked; it is removed on the next run.
	os.Remove(backup)
	return nil
}
