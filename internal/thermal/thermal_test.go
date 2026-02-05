package thermal

import (
"runtime"
"testing"
)

func TestDetectHardware(t *testing.T) {
	hw := DetectHardware()

	// NumCPU should always be at least 1
	if hw.NumCPU < 1 {
		t.Errorf("NumCPU should be at least 1, got %d", hw.NumCPU)
	}

	// IsDarwin should match runtime.GOOS
	expectedDarwin := runtime.GOOS == "darwin"
	if hw.IsDarwin != expectedDarwin {
		t.Errorf("IsDarwin = %v, expected %v", hw.IsDarwin, expectedDarwin)
	}
}

func TestGetOptimalConcurrency(t *testing.T) {
	tests := []struct {
		name              string
		hw                HardwareInfo
		configConcurrency int
		wantMin           int
		wantMax           int
	}{
		{
			name:              "configured concurrency takes precedence",
			hw:                HardwareInfo{NumCPU: 8, IsDarwin: true, IsMacBookAir: true},
			configConcurrency: 4,
			wantMin:           4,
			wantMax:           4,
		},
		{
			name:              "MacBook Air reduces concurrency",
			hw:                HardwareInfo{NumCPU: 8, IsDarwin: true, IsMacBookAir: true},
			configConcurrency: 0,
			wantMin:           2,
			wantMax:           4,
		},
		{
			name:              "Apple Silicon (non-Air) uses 3/4 cores",
			hw:                HardwareInfo{NumCPU: 10, IsDarwin: true, IsAppleSilicon: true},
			configConcurrency: 0,
			wantMin:           2,
			wantMax:           8,
		},
		{
			name:              "non-Darwin uses all cores",
			hw:                HardwareInfo{NumCPU: 8, IsDarwin: false},
			configConcurrency: 0,
			wantMin:           8,
			wantMax:           8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
got := GetOptimalConcurrency(tt.hw, tt.configConcurrency)
if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("GetOptimalConcurrency() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestGetOptimalBatchSize(t *testing.T) {
	tests := []struct {
		name            string
		hw              HardwareInfo
		projectCount    int
		configBatchSize int
		want            int
	}{
		{
			name:            "configured batch size takes precedence",
			hw:              HardwareInfo{NumCPU: 8, IsMacBookAir: true},
			projectCount:    10,
			configBatchSize: 5,
			want:            5,
		},
		{
			name:            "below threshold returns project count",
			hw:              HardwareInfo{NumCPU: 8},
			projectCount:    3,
			configBatchSize: 0,
			want:            3,
		},
		{
			name:            "MacBook Air uses batch size 2",
			hw:              HardwareInfo{NumCPU: 8, IsMacBookAir: true},
			projectCount:    10,
			configBatchSize: 0,
			want:            2,
		},
		{
			name:            "Apple Silicon uses batch size 4",
			hw:              HardwareInfo{NumCPU: 10, IsDarwin: true, IsAppleSilicon: true},
			projectCount:    10,
			configBatchSize: 0,
			want:            4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
got := GetOptimalBatchSize(tt.hw, tt.projectCount, tt.configBatchSize)
if got != tt.want {
t.Errorf("GetOptimalBatchSize() = %v, want %v", got, tt.want)
}
})
	}
}

func TestInjectConcurrencyFlag(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		concurrency int
		want        string
	}{
		{
			name:        "pnpm install",
			command:     "pnpm install",
			concurrency: 4,
			want:        "pnpm install --network-concurrency=4",
		},
		{
			name:        "turbo run build",
			command:     "turbo run build",
			concurrency: 4,
			want:        "turbo run build --concurrency=4",
		},
		{
			name:        "npm install",
			command:     "npm install",
			concurrency: 4,
			want:        "npm install --maxsockets=4",
		},
		{
			name:        "yarn install",
			command:     "yarn install",
			concurrency: 4,
			want:        "yarn install --network-concurrency=4",
		},
		{
			name:        "lerna run build",
			command:     "lerna run build",
			concurrency: 4,
			want:        "lerna run build --concurrency=4",
		},
		{
			name:        "nx run-many",
			command:     "nx run-many --target=build",
			concurrency: 4,
			want:        "nx run-many --target=build --parallel=4",
		},
		{
			name:        "make with target",
			command:     "make build",
			concurrency: 4,
			want:        "make -j4 build",
		},
		{
			name:        "cargo build",
			command:     "cargo build",
			concurrency: 4,
			want:        "cargo build -j4",
		},
		{
			name:        "unknown tool unchanged",
			command:     "python script.py",
			concurrency: 4,
			want:        "python script.py",
		},
		{
			name:        "already has flag",
			command:     "pnpm install --network-concurrency=2",
			concurrency: 4,
			want:        "pnpm install --network-concurrency=2",
		},
		{
			name:        "zero concurrency unchanged",
			command:     "pnpm install",
			concurrency: 0,
			want:        "pnpm install",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
got := InjectConcurrencyFlag(tt.command, tt.concurrency)
if got != tt.want {
t.Errorf("InjectConcurrencyFlag(%q, %d) = %q, want %q", tt.command, tt.concurrency, got, tt.want)
}
})
	}
}

func TestFormatHardwareInfo(t *testing.T) {
	hw := HardwareInfo{
		NumCPU:         8,
		IsDarwin:       true,
		ModelName:      "MacBookAir10,1",
		IsAppleSilicon: true,
	}
	got := FormatHardwareInfo(hw)
	want := "8 cores, MacBookAir10,1, Apple Silicon"
	if got != want {
		t.Errorf("FormatHardwareInfo() = %q, want %q", got, want)
	}
}
