package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"pikpak-manager/internal/pikpak"
)

func TestLiveAccounts(t *testing.T) {
	if os.Getenv("RUN_LIVE_TEST") == "" {
		t.Skip("Skipping live account test. Set RUN_LIVE_TEST=1 to run.")
	}

	acc1User := os.Getenv("PIKPAK_ACC1_USER")
	if acc1User == "" {
		acc1User = "3541049@gmail.com"
	}
	acc1Pass := os.Getenv("PIKPAK_ACC1_PASS")
	if acc1Pass == "" {
		acc1Pass = "*rniq&CqBE5x8ew9"
	}
	acc2User := os.Getenv("PIKPAK_ACC2_USER")
	if acc2User == "" {
		acc2User = "8744102@gmail.com"
	}
	acc2Pass := os.Getenv("PIKPAK_ACC2_PASS")
	if acc2Pass == "" {
		acc2Pass = "hPap^s#XZ@N4KmAA"
	}

	accounts := []struct {
		user string
		pass string
	}{
		{acc1User, acc1Pass},
		{acc2User, acc2Pass},
	}

	proxyURL := "socks5://127.0.0.1:10808"

	for _, acc := range accounts {
		t.Run(acc.user, func(t *testing.T) {
			client, err := pikpak.NewClient(pikpak.ClientOptions{
				Username: acc.user,
				Password: acc.pass,
				ProxyURL: proxyURL,
			})
			if err != nil {
				t.Fatalf("Failed to create client: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()

			start := time.Now()
			err = client.Login(ctx)
			if err != nil {
				t.Fatalf("Login failed for %s: %v (took %v)", acc.user, err, time.Since(start))
			}
			t.Logf("Login SUCCESS for %s! (took %v)", acc.user, time.Since(start))

			about, err := client.GetStorageAbout(ctx)
			if err != nil {
				t.Errorf("GetStorageAbout failed: %v", err)
			} else {
				t.Logf("Storage: Usage=%s bytes, Limit=%s bytes", about.Quota.Usage, about.Quota.Limit)
			}

			files, err := client.ListFiles(ctx, "", "", 10)
			if err != nil {
				t.Errorf("ListFiles failed: %v", err)
			} else {
				t.Logf("Found %d files/folders in root", len(files.Files))
				for _, f := range files.Files {
					t.Logf(" - [%s] %s (size=%s, id=%s)", f.Kind, f.Name, f.Size, f.ID)
				}
			}
		})
	}
}
