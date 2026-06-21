package config

import (
	"testing"
	"time"
)

func TestIsLegacyConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{
			name: "Legacy config with items",
			config: Config{
				Items:   []string{"Blanton's"},
				Zipcode: "97201",
				Users:   []UserConfig{},
			},
			expected: true,
		},
		{
			name: "Legacy config with notifications",
			config: Config{
				Notifications: []NotificationConfig{{Type: "gotify"}},
				Users:         []UserConfig{},
			},
			expected: true,
		},
		{
			name: "Multi-user config",
			config: Config{
				Users: []UserConfig{{Name: "user1"}},
			},
			expected: false,
		},
		{
			name: "Empty config",
			config: Config{
				Users: []UserConfig{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLegacyConfig(tt.config)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMigrateLegacyConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		expectName  string
	}{
		{
			name: "Valid legacy config",
			config: Config{
				Items:    []string{"Blanton's", "Weller"},
				Zipcode:  "97201",
				Distance: 15,
				Interval: 6 * time.Hour,
				Verbose:  true,
				Notifications: []NotificationConfig{
					{Type: "gotify", Endpoint: "https://gotify.example.com"},
				},
			},
			expectError: false,
			expectName:  "default",
		},
		{
			name: "Legacy config without items",
			config: Config{
				Zipcode:  "97201",
				Distance: 15,
			},
			expectError: true,
		},
		{
			name: "Legacy config without zipcode",
			config: Config{
				Items:    []string{"Blanton's"},
				Distance: 15,
			},
			expectError: true,
		},
		{
			name: "Legacy config with zero distance gets default",
			config: Config{
				Items:    []string{"Blanton's"},
				Zipcode:  "97201",
				Distance: 0,
				Interval: 6 * time.Hour,
			},
			expectError: false,
			expectName:  "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := migrateLegacyConfig(tt.config)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if len(result.Users) != 1 {
					t.Errorf("Expected 1 user, got %d", len(result.Users))
				}
				if result.Users[0].Name != tt.expectName {
					t.Errorf("Expected user name %q, got %q", tt.expectName, result.Users[0].Name)
				}
				// Check that distance gets default value if zero
				if tt.config.Distance == 0 && result.Users[0].Distance != 10 {
					t.Errorf("Expected default distance 10, got %d", result.Users[0].Distance)
				}
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid config",
			config: Config{
				Users: []UserConfig{
					{
						Name:     "user1",
						Items:    []string{"Blanton's"},
						Zipcode:  "97201",
						Distance: 10,
					},
				},
			},
			expectError: false,
		},
		{
			name: "No users",
			config: Config{
				Users: []UserConfig{},
			},
			expectError: true,
			errorMsg:    "at least one user must be configured",
		},
		{
			name: "User without name",
			config: Config{
				Users: []UserConfig{
					{
						Items:    []string{"Blanton's"},
						Zipcode:  "97201",
						Distance: 10,
					},
				},
			},
			expectError: true,
			errorMsg:    "must have a name",
		},
		{
			name: "User without items",
			config: Config{
				Users: []UserConfig{
					{
						Name:     "user1",
						Zipcode:  "97201",
						Distance: 10,
					},
				},
			},
			expectError: true,
			errorMsg:    "must have at least one item",
		},
		{
			name: "User without zipcode",
			config: Config{
				Users: []UserConfig{
					{
						Name:     "user1",
						Items:    []string{"Blanton's"},
						Distance: 10,
					},
				},
			},
			expectError: true,
			errorMsg:    "must have a zipcode",
		},
		{
			name: "User with zero distance",
			config: Config{
				Users: []UserConfig{
					{
						Name:     "user1",
						Items:    []string{"Blanton's"},
						Zipcode:  "97201",
						Distance: 0,
					},
				},
			},
			expectError: true,
			errorMsg:    "must have a positive distance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestNotificationConfigCondenseField(t *testing.T) {
	// Test that the Condense field is properly included in NotificationConfig
	notification := NotificationConfig{
		Type:     "gotify",
		Endpoint: "https://gotify.example.com",
		Condense: true,
		Credential: map[string]string{
			"token": "test_token",
		},
	}

	if !notification.Condense {
		t.Errorf("Expected Condense to be true, got false")
	}

	// Test default value (should be false)
	defaultNotification := NotificationConfig{
		Type:     "slack",
		Endpoint: "https://slack.example.com",
	}

	if defaultNotification.Condense {
		t.Errorf("Expected default Condense to be false, got true")
	}
}

func TestUserConfigStructure(t *testing.T) {
	// Test that UserConfig has all required fields
	user := UserConfig{
		Name:     "test_user",
		Items:    []string{"Blanton's", "Weller"},
		Zipcode:  "97201",
		Distance: 15,
		Notifications: []NotificationConfig{
			{
				Type:     "gotify",
				Endpoint: "https://gotify.example.com",
				Condense: true,
				Credential: map[string]string{
					"token": "test_token",
				},
			},
		},
	}

	if user.Name != "test_user" {
		t.Errorf("Expected Name to be 'test_user', got %q", user.Name)
	}

	if len(user.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(user.Items))
	}

	if user.Zipcode != "97201" {
		t.Errorf("Expected Zipcode to be '97201', got %q", user.Zipcode)
	}

	if user.Distance != 15 {
		t.Errorf("Expected Distance to be 15, got %d", user.Distance)
	}

	if len(user.Notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(user.Notifications))
	}

	if !user.Notifications[0].Condense {
		t.Errorf("Expected notification Condense to be true, got false")
	}
}

func TestMultiUserConfigStructure(t *testing.T) {
	// Test that Config supports multiple users
	config := Config{
		Interval:  6 * time.Hour,
		UserAgent: "test-agent",
		Verbose:   true,
		Users: []UserConfig{
			{
				Name:     "user1",
				Items:    []string{"Blanton's"},
				Zipcode:  "97201",
				Distance: 10,
				Notifications: []NotificationConfig{
					{Type: "gotify", Condense: false},
				},
			},
			{
				Name:     "user2",
				Items:    []string{"Weller"},
				Zipcode:  "97210",
				Distance: 15,
				Notifications: []NotificationConfig{
					{Type: "slack", Condense: true},
				},
			},
		},
	}

	if len(config.Users) != 2 {
		t.Errorf("Expected 2 users, got %d", len(config.Users))
	}

	if config.Users[0].Name != "user1" {
		t.Errorf("Expected first user name to be 'user1', got %q", config.Users[0].Name)
	}

	if config.Users[1].Name != "user2" {
		t.Errorf("Expected second user name to be 'user2', got %q", config.Users[1].Name)
	}

	// Test that global settings are preserved
	if config.Interval != 6*time.Hour {
		t.Errorf("Expected Interval to be 6h, got %v", config.Interval)
	}

	if !config.Verbose {
		t.Errorf("Expected Verbose to be true, got false")
	}
}

func TestConfigFileLoadingBehavior(t *testing.T) {
	// Test that the config loading logic properly handles custom config files
	// This test verifies the comment in GetConfig about only loading default config.yaml
	// when no custom config file was set via CLI

	// The actual behavior is tested through the viper.ConfigFileUsed() check
	// This test documents the expected behavior

	// When viper.ConfigFileUsed() returns empty string, default config.yaml should be loaded
	// When viper.ConfigFileUsed() returns a file path, default config.yaml should NOT be loaded

	// This is a documentation test to ensure the behavior is clear
	t.Log("Config loading behavior:")
	t.Log("- If no custom config file is set via CLI, load default config.yaml if it exists")
	t.Log("- If custom config file is set via CLI, do NOT load default config.yaml")
	t.Log("- This prevents double-loading when user specifies a custom config file")
}

func TestCommonItemStructure(t *testing.T) {
	item := CommonItem{
		Code: "99900046075",
		Name: "Bacardi Superior Rum",
	}

	if item.Code != "99900046075" {
		t.Errorf("Expected Code '99900046075', got %q", item.Code)
	}

	if item.Name != "Bacardi Superior Rum" {
		t.Errorf("Expected Name 'Bacardi Superior Rum', got %q", item.Name)
	}
}

func TestConfigCommonItemsField(t *testing.T) {
	config := Config{
		Interval: 6 * time.Hour,
		CommonItems: []CommonItem{
			{Code: "99900046075", Name: "Bacardi Superior Rum"},
			{Code: "99900014675", Name: "Jack Daniels #7 Whiskey"},
		},
		Users: []UserConfig{
			{Name: "user1", Items: []string{"Blanton's"}, Zipcode: "97201", Distance: 10},
		},
	}

	if len(config.CommonItems) != 2 {
		t.Errorf("Expected 2 common items, got %d", len(config.CommonItems))
	}

	if config.CommonItems[0].Code != "99900046075" {
		t.Errorf("Expected first common item code '99900046075', got %q", config.CommonItems[0].Code)
	}

	if config.CommonItems[1].Name != "Jack Daniels #7 Whiskey" {
		t.Errorf("Expected second common item name 'Jack Daniels #7 Whiskey', got %q", config.CommonItems[1].Name)
	}
}

func TestConfigCommonItemsEmpty(t *testing.T) {
	config := Config{
		Interval: 6 * time.Hour,
		Users: []UserConfig{
			{Name: "user1", Items: []string{"Blanton's"}, Zipcode: "97201", Distance: 10},
		},
	}

	if len(config.CommonItems) != 0 {
		t.Errorf("Expected 0 common items when not configured, got %d", len(config.CommonItems))
	}
}
