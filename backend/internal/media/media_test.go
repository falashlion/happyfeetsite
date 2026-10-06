package media

import (
	"testing"

	"github.com/happyfeet/api/pkg/config"
)

func cloudinaryTestConfig(cloud, key, secret string) config.CloudinaryConfig {
	return config.CloudinaryConfig{CloudName: cloud, APIKey: key, APISecret: secret}
}

// Cloudinary's own worked example from its upload-signature documentation:
// public_id=sample, timestamp=1315060510, api_secret=abcd. Getting this byte
// for byte right is the whole feature — a wrong signature is rejected by
// Cloudinary with a generic 401 that says nothing about which part was wrong.
func TestSignParamsMatchesCloudinaryExample(t *testing.T) {
	got := signParams(map[string]string{
		"public_id": "sample",
		"timestamp": "1315060510",
	}, "abcd")

	const want = "c3470533147774275dd37996cc4d0e68fd03cd4f"
	if got != want {
		t.Errorf("signParams = %q, want %q", got, want)
	}
}

// Parameters must be sorted alphabetically before signing, so the order they
// happen to be inserted in must not change the result.
func TestSignParamsIsOrderIndependent(t *testing.T) {
	a := signParams(map[string]string{
		"timestamp": "1700000000",
		"folder":    "happyfeet/product_image",
		"public_id": "abc",
	}, "s3cret")

	b := signParams(map[string]string{
		"public_id": "abc",
		"timestamp": "1700000000",
		"folder":    "happyfeet/product_image",
	}, "s3cret")

	if a != b {
		t.Errorf("signature depends on map iteration order: %q != %q", a, b)
	}
}

func TestSignParamsChangesWithSecret(t *testing.T) {
	params := map[string]string{"public_id": "x", "timestamp": "1"}
	if signParams(params, "one") == signParams(params, "two") {
		t.Error("different secrets produced the same signature")
	}
}

func TestCloudinaryEnabledRequiresEveryField(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cloud string
		key   string
		sec   string
		want  bool
	}{
		{"all set", "demo", "123", "abc", true},
		{"no cloud name", "", "123", "abc", false},
		{"no api key", "demo", "", "abc", false},
		{"no secret", "demo", "123", "", false},
		{"empty", "", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := cloudinaryTestConfig(tc.cloud, tc.key, tc.sec)
			if got := c.Enabled(); got != tc.want {
				t.Errorf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}
