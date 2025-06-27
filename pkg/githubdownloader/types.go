package githubdownloader

// RepoMetadata holds metadata about a downloaded GitHub repository.
type RepoMetadata struct {
	SourceURL    string
	CommitHash   string
	DownloadedAt int64 // Unix timestamp
}

// DownloaderInterface defines the methods for downloading and caching repositories.
type DownloaderInterface interface {
	DownloadRepo(url string) (string, *RepoMetadata, error)
	ClearExpiredCache() error
	GetCacheStats() (totalFiles, validFiles, expiredFiles int, err error)
	// Add more methods as needed
}
