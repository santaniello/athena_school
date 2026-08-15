package profile

// Store persists the local UserProfile.
type Store interface {
	Save(profile UserProfile) error
	Load() (UserProfile, error)
}
