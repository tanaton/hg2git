package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	fast_export_path = "/tmp/fast-export/hg-fast-export.sh"
)

type repo struct {
	name  string
	email string
}

func main() {
	ctx := context.Background()
	// 全体設定
	r, err := global(ctx)
	if err != nil {
		log.Fatal(err)
	}
	err = r.check(ctx, ".")
	if err != nil {
		log.Fatal(err)
	}
}

func global(ctx context.Context) (*repo, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*60)
	defer cancel()
	err := command(ctx, "git", "config", "--global", "core.ignoreCase", "false")
	if err != nil {
		return nil, err
	}
	err = command(ctx, "git", "config", "--global", "core.quotepath", "false")
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "git", "config", "--global", "user.name")
	name, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	cmd = exec.CommandContext(ctx, "git", "config", "--global", "user.email")
	email, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return &repo{
		name:  strings.Trim(string(name), " \n"),
		email: strings.Trim(string(email), " \n"),
	}, nil

}

func (r *repo) check(ctx context.Context, cur string) error {
	dl, err := os.ReadDir(cur)
	if err != nil {
		return err
	}
	f := func(dl []fs.DirEntry, p string) bool {
		for _, it := range dl {
			if it.Name() == p {
				return true
			}
		}
		return false
	}
	if f(dl, ".hg") {
		return r.create(ctx, cur)
	}
	for _, de := range dl {
		if de.IsDir() {
			err := r.check(ctx, filepath.Join(cur, de.Name()))
			if err != nil {
				log.Println(err)
			}
		}
	}
	return nil
}

func (r *repo) create(ctx context.Context, cur string) error {
	prev, err := filepath.Abs(".")
	if err != nil {
		return err
	}
	defer os.Chdir(prev)
	// ディレクトリ移動
	err = os.Chdir(cur)
	if err != nil {
		return err
	}

	err = os.RemoveAll(".git")
	if err != nil {
		return err
	}
	err = command(ctx, "git", "init")
	if err != nil {
		return err
	}
	// ユーザー変換テーブル
	authors := "authors.txt"
	err = r.author(ctx, authors)
	if err != nil {
		return err
	}
	defer os.Remove(authors)
	err = command(ctx, "sh", fast_export_path, "-r", ".", "--force", "--fe", "cp932", "-A", authors)
	if err != nil {
		return err
	}
	err = command(ctx, "git", "checkout", "main", "--force")
	if err != nil {
		return err
	}
	return nil
}

func (r *repo) author(ctx context.Context, p string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()

	buf := bytes.Buffer{}
	cmd := exec.CommandContext(ctx, "hg", "log", "-T", "{author}\n")
	cmd.Stdout = &buf
	err := cmd.Run()
	if err != nil {
		return err
	}

	m := make(map[string]struct{}, 4)
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		m[strings.Trim(scanner.Text(), " \n")] = struct{}{}
	}

	// 変換テーブルが不要でもファイルは用意する
	fp, err := os.Create(p)
	if err != nil {
		return err
	}
	defer fp.Close()
	for key := range m {
		fmt.Fprintf(fp, "\"%s\"=\"%s <%s>\"\n", key, r.name, r.email)
	}
	return nil
}

func command(ctx context.Context, command string, args ...string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd.Run()
}
