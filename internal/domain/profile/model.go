package profile

// FetchProfileInput represents the input for fetching a user's profile.
type FetchProfileInput struct {
	InstanceID string `json:"instanceId"`
	Number     string `json:"number"`
}

// FetchBusinessProfileInput represents the input for fetching a business profile.
type FetchBusinessProfileInput struct {
	InstanceID string `json:"instanceId"`
	Number     string `json:"number"`
}

// UpdateProfileNameInput represents the input for updating the profile name.
type UpdateProfileNameInput struct {
	InstanceID string `json:"instanceId"`
	Name       string `json:"name"`
}

// Profile represents a WhatsApp user profile.
type Profile struct {
	WID        string `json:"wid,omitempty"`
	Name       string `json:"name,omitempty"`
	PictureURL string `json:"pictureUrl,omitempty"`
	Status     string `json:"status,omitempty"`
}

// BusinessProfile represents a WhatsApp business profile.
type BusinessProfile struct {
	WID         string `json:"wid,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
	Email       string `json:"email,omitempty"`
	Website     string `json:"website,omitempty"`
	Address     string `json:"address,omitempty"`
	PictureURL  string `json:"pictureUrl,omitempty"`
}

// UpdateProfileNameOutput represents the response after updating profile name.
type UpdateProfileNameOutput struct {
	Update string `json:"update"`
}

// UpdateProfileStatusInput represents the input for updating profile status.
type UpdateProfileStatusInput struct {
	InstanceID string `json:"instanceId"`
	Status     string `json:"status"`
}

// UpdateProfileStatusOutput represents the response after updating profile status.
type UpdateProfileStatusOutput struct {
	Update string `json:"update"`
}

// UpdateProfilePictureInput represents the input for updating profile picture.
type UpdateProfilePictureInput struct {
	InstanceID string `json:"instanceId"`
	Picture    string `json:"picture"`
}

// UpdateProfilePictureOutput represents the response after updating profile picture.
type UpdateProfilePictureOutput struct {
	Update string `json:"update"`
}

// RemoveProfilePictureInput represents the input for removing profile picture.
type RemoveProfilePictureInput struct {
	InstanceID string `json:"instanceId"`
}

// RemoveProfilePictureOutput represents the response after removing profile picture.
type RemoveProfilePictureOutput struct {
	Update string `json:"update"`
}

// FetchPrivacySettingsInput represents the input for fetching privacy settings.
type FetchPrivacySettingsInput struct {
	InstanceID string `json:"instanceId"`
}

// PrivacySettings represents WhatsApp privacy settings.
type PrivacySettings struct {
	ReadReceipts string `json:"readreceipts"`
	Profile      string `json:"profile"`
	Status       string `json:"status"`
	Online       string `json:"online"`
	Last         string `json:"last"`
	GroupAdd     string `json:"groupadd"`
}

// UpdatePrivacySettingsInput represents the input for updating privacy settings.
type UpdatePrivacySettingsInput struct {
	InstanceID   string `json:"instanceId"`
	ReadReceipts string `json:"readreceipts"`
	Profile      string `json:"profile"`
	Status       string `json:"status"`
	Online       string `json:"online"`
	Last         string `json:"last"`
	GroupAdd     string `json:"groupadd"`
}

// UpdatePrivacySettingsOutput represents the response after updating privacy settings.
type UpdatePrivacySettingsOutput struct {
	Update string `json:"update"`
}
