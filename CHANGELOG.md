# Changelog

All notable changes to PixelHop will be documented in this file.

## [2.2.0] - 2025-12-18

### Added
- **View Count Tracking**: Track views for all images
  - View count increments once per hour per IP
  - View count and last viewed date in Image Info
  - "Most Views" sort option in galleries
  - Total Views stat in galleries
- **Report Image Feature**: Users can report inappropriate images
  - Report button on image view page
  - Multiple report reasons (NSFW, Illegal, Privacy, Copyright, Other)
  - Report queue in Admin Security panel
  - Dismiss or Delete actions for admins
- **User Account Status System**:
  - **Active**: Normal access
  - **Locked**: Cannot login, images still viewable, must contact support
  - **Suspended**: No access, images inaccessible, auto-deleted in 30 days
- **Admin Warning System**: Send warnings that popup on user's next login
- **Image Expiration System**: Auto-delete images after 90 days of inactivity
  - Daily cron job at 2 AM
  - Based on last_viewed or created_at date
  - Logged to data/expiration_log.json

### Changed
- **Admin Gallery Stats**: Compact layout (5 stats in 1 row)
- **Image View Page**: Added view count, last viewed, and report button
- **Admin Users Page**: Added status management (Lock/Suspend/Warning)
- **Login Flow**: Checks account status before allowing login

### Fixed
- Modal styling for light mode in admin panel
- Report dismiss now properly disables row
- PHP 8.5 compatibility improvements

## [2.1.0] - 2025-12-18

### Added
- **Hybrid Storage System**: R2 + Contabo S3 integration
  - Cloudflare R2 for thumbnails (9.5GB free tier)
  - Contabo S3 for original files (250GB)
  - Automatic failover between providers
- **R2StorageManager**: Smart storage routing with rate limiting
- **Storage Stats Dashboard**: Real-time monitoring for R2, Contabo, and Temp storage
- **Security Firewall**: IP blocking, rate limiting, abuse detection
- **Swap Memory**: 4GB swap with swappiness=10 for better performance

### Changed
- **Admin Dashboard**: Consolidated from 10 menus to 5 (Dashboard, Users, Gallery, Security, Settings)
- **Admin UI**: New shared header, CSS, and JS components
- **Storage Display**: Progress bars for R2, Contabo, and Temp (10GB allocation)
- **Gauge Charts**: Larger donut charts (130px) with better centering

### Fixed
- Member settings page layout and styling
- Admin panel centering with visible background
- Storage stats now show actual provider usage

## [2.0.0] - 2025-12-17

### Added
- Admin panel v2.0 with glassmorphism design
- Theme toggle (dark/light mode)
- Real-time server health monitoring
- Python process management for OCR/RemBG

### Changed
- Complete admin UI overhaul
- Responsive design improvements

## [1.0.0] - 2025-12-16

### Added
- Initial release
- Image upload with multiple formats
- Image tools: resize, crop, compress, convert
- OCR text extraction
- Background removal (RemBG)
- User authentication with Google OAuth
- Rate limiting and abuse protection
