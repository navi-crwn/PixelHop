# Changelog

What changed in PixelHop, newest release first. Each entry says what changed, where you see it, what it means for you and why it was done. Technical notes live in the GitHub release of each version.

## [Unreleased]

## [3.11.4] - 2026-11-14 - Dashboard polish

### Added
- **Dashboard.** When the site owner posts an announcement it now appears as a card at the top of your dashboard, with a Close button. A closed announcement stays hidden for a week, and comes back if the wording changes. Before, the announcement was a bar across the page that the sidebar covered, so people often missed it.

### Changed
- **Studio, Help, About, Changelog and other tool and information pages.** If you are signed in, the Back link, the logo and the Upload buttons now take you to your dashboard instead of the public home page. Visitors who are not signed in still go to the home page. People kept landing on the home page after using the sidebar, which was confusing.
- **Studio.** Opened from the dashboard, the Photo studio now appears inside the dashboard, with the sidebar and top bar around it, so it feels like part of the dashboard. On medium screens its three columns become one so the picture area stays wide enough to work in.
- **Dashboard sidebar.** The line "Part of Hop Easy Link" is now the last line at the bottom of the sidebar instead of the middle of the page footer, where it sometimes ended up in the wrong place.
- **Top bar on phones.** On a 375 pixel wide phone the top bar no longer splits into two rows, and the language, account and family menus open fully on the screen instead of running off the right edge. Reported by the site owner.
- **Changelog page.** The changelog now lists one plain-language entry per release, grouped as added, changed, fixed and so on, with a Technical details link to the full notes. Before, it listed raw code changes, which most people could not read. The page is written from this file and checked by the project's tests, so it stays readable.
- **Dashboard search.** In German and Russian the search row no longer pushes the page sideways on a phone, because its buttons now wrap onto a second line when there is no room.

## [3.11.3] - 2026-11-14 - Daily AI limit that sticks

### Fixed
- **AI tools in the Studio.** The daily limit now survives a restart of the server. Before, restarting PixelHop gave the whole day back, so the limit could be used twice. Several images sent at the same moment can no longer go over the limit, and the use is still given back when the AI service fails.
- **Settings, Security and sessions.** Trying to change the password of a HOP ID account through a hand-made request is now refused. The password belongs to the HOP ID hub, and the form was already hidden. Nothing changes and other browsers stay signed in.
- **Error pages and refused forms.** A page that does not exist, a failed sign-in or a form with a wrong value lost some of its safety and caching settings, because the answer was started too early. They now keep them.

### Changed
- **Public pages.** Visitors now always check back with PixelHop before a stored copy of a page is shown, so a shared cache can no longer show one visitor's language to the next visitor. Signed-in pages, images and the status page are unchanged.
- **AI tools.** The tools tell how much of the day's share is left, both on the result and when the share is used up, so programs and scripts can show it.

## [3.11.2] - 2026-11-13 - Settings tabs and a calmer dashboard

### Added
- **Upload progress.** One line above the results now says "Uploading 3 of 12 files, 40 percent" for the whole batch, in all nine languages. Before, only each single file showed progress.
- **Settings.** Settings is split into tabs: Profile, Security and sessions, API and integrations, Data, and Delete. Every link PixelHop gives out opens the tab that holds its card. A page that answers a form, such as a new API key, a new webhook secret or an error, now opens the tab where that form is, so a one-time key is on screen when it is made. Before, that page opened on Profile and hid the key. On a phone the tabs wrap onto two rows.

### Changed
- **Dashboard.** After an upload you stay on the dashboard. A batch of more than three files shows a banner that links to the results page, instead of moving the browser there. "Upload more" brings a member back to the dashboard uploader.
- **Dashboard.** The seven selection buttons became one quiet Actions menu: copy links, ZIP, make album, show or hide on profile, and delete. The menu returns the keyboard to its button when it closes.
- **Dashboard.** The extra row of member tabs is gone, the sidebar is the menu, and the top bar always has an Upload button. The top bar is solid and sticks to the very top edge, so page text no longer shows through or above it while scrolling.

### Removed
- **Studio.** The QR code tool is removed, because QR codes are now LinkHop's job. A copy connected to HEL.ink shows one quiet link to LinkHop's QR page. The QR code on each image page stays.
- **Explore.** The Explore page, its menu links and its sitemap entry are gone, and the old address goes to the home page. Public profiles and the option to show a file on your profile are unchanged.

## [3.11.1] - 2026-11-12 - Daily AI limit, reminders and search engines

### Fixed
- **Studio, AI tools.** The daily limit used to last only about an hour, because old counts were cleaned away too early. It now lasts the full day, and calls that fail or are refused for the file itself no longer count against it.
- **Settings, API and integrations.** HOP ID members could not see the Webhooks card although the documentation sends people there. It is back. Only the password form stays hidden, because that password belongs to the hub.
- **Studio, Watermark.** A Remove logo button now brings back the text option after a logo was chosen.
- **Studio.** Download and Upload result appear only when there is a result to save. Before they also showed in tools that make no picture, such as the color palette.
- **Studio, Magic eraser.** It no longer promises about 10 seconds. It says it can take longer when the server is busy, because it works on the server one image at a time.

### Added
- **Studio, Image to text.** A short note says that machine-read text can be wrong and should be checked.
- **Dashboard.** The "Get the most out of PixelHop" card can be hidden for a week, like the reminder cards in the other HOP apps. It comes back while steps are open and is gone when all four are done.
- **Theme and language.** Your theme and language now follow you across all HOP apps, so you choose them once.
- **Search engines.** Pages now name their one true address and, for the fixed public pages, their nine language versions, so search results show the right page in the right language.

### Changed
- **Search engines.** Sign-in, account, dashboard, trash and upload result pages, and files behind a content warning, are now kept out of search results. Public images, albums and profiles stay findable.

### Removed
- **Dashboard.** The shortcut to HEL.ink short links is gone, because short links belong to LinkHop and the sidebar already lists the HOP family.

## [3.11.0] - 2026-11-12 - Claim your guest uploads

### Added
- **Dashboard and home page.** If you uploaded files before signing in, PixelHop now says so and offers a Claim button. Claiming moves those files into your library, checks the private delete link of each one, respects your storage space and skips files that were removed, reported or have expired. Before, files uploaded as a guest stayed separate from your account after you signed in.
- **Saving from FileHop.** FileHop's "Save to PixelHop" button can now store a finished picture or PDF in the library of the signed-in person. It uses the same size limits, safety checks and storage space as a normal upload, and it stays off until the site owner switches it on.

## [3.10.0] - 2026-11-11 - Guest uploads expire, safer account switching

### Added
- **Upload page.** New guest uploads are deleted after 30 days unless the guest chose a shorter time, and the upload form says so in all nine languages. Uploads by signed-in members and files already stored are not touched. This keeps the server from filling up with forgotten files.

### Changed
- **Signing in with HOP ID.** If you switch accounts at the hub, PixelHop no longer stays on the old account. A few minutes later, it quietly checks with the hub: the same account carries on, a different account replaces the session, and someone signed out at the hub is signed out here. Uploads and downloads are never interrupted by this check.
- **Public pages.** Fixed pages such as legal, help, about and the studio now carry the site's picture, so a pasted link shows a card.
- **Home page.** The small numbered labels above the headings are gone, and the row of HOP family tiles keeps five tiles in a row down to tablet width, then wraps neatly on phones.
- **HOP family.** PayHop joins the family list, the footer, the FAQ, the README and the project website, in all nine languages. The brand fonts and pictures now come from the family's shared set.
- **Signing in with HOP ID.** The hub counts as reachable only when it really answers. While it does not, nobody is signed out or locked out.

## [3.9.0] - 2026-11-10 - Shared footer, dialogs, trash and tougher safety

### Added
- **Footer.** Every footer now starts with the HOP ID mark and "Part of Hop Easy Link" linking to hel.ink.
- **Questions and notices.** Questions such as "Delete this file?" now use the family's shared dialog instead of the browser's own, with the question and its explanation shown separately.
- **Upload result page.** A ticket's private note is shown once to its owner and stays visible only to the owner and the moderators.
- **Dashboard.** The trash has its own entry in the sidebar.
- **Interface style.** An optional Clean style with flat surfaces and quiet tables can be chosen. The usual HOP look stays the default.
- **FileHop in the family.** FileHop is listed in the sidebar, the app switcher, the footer and the About page, with its texts in all nine languages.
- **Picker for BioHop.** BioHop's "Choose from PixelHop" button can now list a member's own pictures and PDFs, leaving out anything private, protected, expiring, hidden or behind a content warning. The files stay here and the other app only keeps their address.
- **Account export, API keys and sign-outs.** API keys can be limited to uploads only, owners can refuse embedding of their images on other websites, Settings lists where you are signed in and can sign out everywhere else, and everything you have can be exported as one ZIP.
- **Albums.** An album can be embedded on another website, and the album page copies the code.
- **Keyboard shortcuts.** Press the question mark key for shortcuts: upload, studio, dashboard, search and copy link.
- **Trash and tags.** Deleted files go to a trash for 7 days first, and files can have tags that you can search and filter by.
- **Installers.** The install script checks the computer, asks a few questions, builds PixelHop and can set it up as a service or in Docker. The update script makes a backup first, and the uninstall script keeps your data unless asked.
- **Maintenance mode.** The site owner can pause uploads while everything else keeps working.
- **Dashboard.** A "Get the most out of PixelHop" card shows four steps, with a progress ring.
- **Home page.** The part of the home page below the upload box was rebuilt with scroll scenes that show an upload, the removal of location and camera data, an album turning into five link formats and a photo that disappears when its time runs out. Everything is complete without scripts or with reduced motion.
- **Pages.** New About, Features, FAQ and Contact pages and a live Status page, all in the sitemap.
- **Content warnings.** An upload or album can be marked sensitive or adult. Visitors see it blurred and confirm before it opens.
- **Storage guard.** Uploads pause with a clear message while the server is almost out of disk space, and the upload pages check again every 30 seconds.

### Fixed
- **Home page.** All five family tiles now assemble, and the section draws every other app, FileHop included, with its name and host.
- **Signing out.** Signing out of a HOP ID account now returns you to PixelHop, and signing out at HOP ID, LinkHop or BioHop ends the PixelHop session as well. Before, the next visit signed you straight back in.
- **Changelog.** The page lists every merged change again.
- **Trash.** A file its owner put in the trash now answers as not found, and hidden media is no longer cached publicly.
- **Language menu.** It stays on the screen on a 375 pixel phone and closes with the Escape key.
- **Profile photos.** A profile photo from HOP ID or Google now shows correctly, and someone who hides their profile still sees their own photo.
- **Settings and pages.** The home page's Batch editing button opened a page that does not exist and now opens the batch page. Settings no longer offers a profile page link while the profile page is switched off. The image page no longer scrolls sideways on phones. The studio QR tool tells you when the server refuses its text.
- **Install script.** The dry run no longer creates folders.
- **GIF images.** The image page no longer says "Metadata: clean" for GIFs, which are stored as uploaded.
- **Accounts.** An account is no longer deleted when its pictures could not be listed, deleting an account also deletes its profile photo and the pictures in its trash, and deleting a picture removes its stored files first. If something goes wrong, nothing is half-deleted and the action can be run again.

### Changed
- **Header and footer.** The mark and the app name are now real text in the app's own typeface instead of a picture.
- **Controls.** Buttons and fields follow the HOP control standard: one corner radius, 36 pixel controls and capped field widths.
- **Translations.** The app, its documentation and its README come in nine languages: English, Chinese, German, Indonesian, Japanese, Portuguese (Brazil), Spanish, French and Russian. Arabic and right-to-left layout are gone, and old language codes still work.
- **Dashboard.** The signed-in pages have the same sidebar and top bar as the other HOP dashboards.
- **Speed.** Views are counted in memory and saved every ten seconds, so a popular image no longer slows the site down.
- **Private files.** Private, password-protected and expiring files are served by PixelHop itself and never redirected to public storage.

### Security
- **Media picker.** The picker for BioHop now uses its own secret. Before, PixelHop had to hold BioHop's sign-in secret, so a leak here could have forged a BioHop sign-in.
- **Profile website.** A website address that opens an encrypted NoteHop note, including its key, is refused and is never stored or shown, so a note key cannot leak through a profile.
- **Sign-in.** A sign-in for an account that is not active is refused before any local account is made, and a return address with a hidden control character is no longer followed.
- **Uploads and storage space.** A member's storage space is checked as one step, so uploads that arrive together cannot all pass the same check.

## [3.7.0] - 2026-10-28 - Links on their own host and the control center

### Added
- **Shared links.** Shared links can live on their own host, such as p.hel.ink, while the app itself moves to pixel.hel.ink like the other HOP apps. Every existing link keeps working, and a visitor who opens a shared page on the app's address is sent to the link host.
- **Control center.** The HOP control center can now look after PixelHop: it reads health, statistics, content and reports, and can suspend or warn a user, hide or delete content and change settings. Every change is written to an audit trail. The sign-in ticket from HOP ID now carries the member's role, username, bio and website.
- **Hidden images.** Images remember why they are hidden, so lifting a suspension brings back only the images the suspension hid.

### Removed
- **Admin pages.** The built-in admin pages are gone, because moderation and site settings now live in the control center. Signed-in admins are sent there.

## [3.6.0] - 2026-10-19 - Many languages in one picture

### Added
- **Studio, Image to text.** It now reads 30 languages and up to five at once, for pictures that mix languages such as an English receipt with Japanese lines. Your choice is remembered.

### Changed
- **Studio, Image to text.** Chinese and Japanese text no longer has spaces between every character.
- **Project website.** The project website is now in seven languages.

## [3.5.3] - 2026-10-16 - Upload limits per person

### Changed
- **Upload page.** Guests can upload any file up to 20 MB. Signed-in members keep 20 MB per picture and now get up to 100 MB per video or PDF. The upload box shows each person their own limits, and a guest whose file is too big is told how far signing in goes.

## [3.5.2] - 2026-10-16 - Fewer silent failures

### Fixed
- **Admin, Users.** Changing someone's storage space no longer gets lost. The value is typed in GB and a value that cannot be read is refused with a message. Every action on a user now says what happened.
- **Dashboard.** Showing, hiding or deleting many files at once reports how many worked, and files that cannot go on a profile are no longer skipped silently.
- **Error messages.** The site gives a clearer message when the server answers with an error page instead of the expected data.
- **Settings, Webhooks.** Send test with no webhook now explains that one is needed first.
- **Video and PDF pages.** They no longer advertise an embed link that did not work.

### Security
- **Settings and admin pages.** Confirmation messages come from short codes, so a crafted link can no longer put its own text on those pages.

## [3.5.1] - 2026-10-16 - Member dashboard

### Added
- **Dashboard.** Members can upload straight from the dashboard, and the links show beside the box. Number cards show files, views, albums and storage, shortcuts lead to the studio, albums, public profile and API, and files can be filtered by type.
- **Member pages.** One member menu now links the dashboard, albums, profile, studio and settings.

### Fixed
- **Upload page.** The upload box fills its column again. Since 3.4.0 it had shrunk to the width of its content.

### Removed
- **Old PHP version.** The PixelHop 2 code was removed from the project. It stays available in its own release, and old links and imports keep working.

## [3.5.0] - 2026-10-16 - One account with HEL.ink

### Added
- **Sign-in and sign-up.** When HEL.ink is connected, the email and password form here also accepts HEL.ink accounts, and signing up here creates the account on HEL.ink too, so one account works in both apps.

### Security
- **Sign-in.** An unconfirmed HEL.ink address never takes over an existing PixelHop profile with the same email.

## [3.4.2] - 2026-10-16 - Smaller QR code

### Changed
- **Image page.** The QR code is now a small row inside the details card, with a download link, so the side column no longer runs far below the image.

## [3.4.1] - 2026-10-16 - Standalone again

### Fixed
- **Upload page.** The guest note was hard to read on the box's colored border and now sits below it. Phones no longer see the Ctrl and V hint.

### Changed
- **Sign-in.** PixelHop works fully on its own. Signing in with HEL.ink is an optional add-on that stays off unless the site owner turns it on.

## [3.4.0] - 2026-10-16 - New upload page

### Changed
- **Upload page.** A shorter headline and one clear upload box. Options fold into an Options chip that shows what is set, and the ticket into a Ticket and note chip.
- **Upload results.** Results appear beside the upload box on desktop and below it on phones. Up to three files show there, and bigger batches open the results page.
- **Top bar.** The Install app button moved to the top bar.
- **Upload history.** Files uploaded from this device are folded by default and grouped by day.

## [3.3.2] - 2026-10-16 - Paste button for phones

### Added
- **Upload page.** A Paste button uploads the picture on the clipboard, for phones that have no Ctrl and V, for example a picture copied inside a chat app. A copied link is fetched like "From a link".

## [3.3.1] - 2026-10-16 - Results page for phones

### Added
- **Upload results.** After uploading several files on a phone you land on a page with just those files, their links, a Copy for chat button and, for guests, the delete links.
- **Upload page.** An Install app button appears when the browser offers it, and a hint for iPhone.

### Changed
- **Upload page.** The Ticket box is always visible for signed-in people, with a sign-in hint for guests.

## [3.3.0] - 2026-10-16 - Videos and PDFs

### Added
- **Upload.** MP4, MOV, WebM, 3GP and PDF files get a page, a preview and a link, like pictures, up to 500 MB per file.
- **Upload.** Videos lose their location data at once and are then shrunk in the background so they play on every phone, and PDFs are compressed. Both need extra programs on the server and are skipped without them.
- **Upload.** Big files are sent in pieces, so a dropped connection can be resumed.
- **Tickets and reports.** An upload can carry a ticket number and a note, and all its files go into one album with that title and note. A Copy for chat button copies the ticket, note, album link and a numbered list of every file's link.
- **Sharing.** The phone share sheet accepts videos and PDFs, and the documentation has a guide for an iPhone Shortcut.

### Changed
- **Upload page.** Upload rows stack their buttons on phones so the link stays readable.

## [3.2.0] - 2026-10-16 - Private files, albums and webhooks

### Added
- **Image page.** Images can be private or password-protected, and the file itself is locked too, so hotlinks stop working. Owners can edit title, description, visibility and profile listing, and see views per day and the sites that sent visitors.
- **Images.** An image can be resized on the fly through its address, and is cached per image.
- **Albums.** An album editor lets you change title, description and cover, reorder by dragging, and add or remove pictures. There is a My albums page and a Make album button for a selection.
- **Explore.** A page shows pictures people chose to show publicly, plus a sitemap, embed support and a public gallery for bio pages and other sites.
- **Settings.** Webhooks announce uploaded and deleted images, signed so the receiver can trust them.
- **Upload.** iPhone HEIC and AVIF pictures are converted on the server, and upload results list the hidden data that was removed.
- **Installable app.** PixelHop can be installed on a phone and accepts pictures from the share sheet.
- **Studio.** A Watermark tool in the studio and in batch, and a Hold to compare button.
- **Command line.** The same program can upload from a terminal.

### Changed
- **Old result page.** It now hides private and password-protected images.

## [3.1.0] - 2026-10-16 - Public profiles

### Added
- **Profiles.** Public profiles with display name, username, profile photo, bio and website, edited in Settings. Every account gets a username automatically.
- **Image and album pages.** They show who shared the file, with a link to their profile, and link previews say "Shared by". Owners can switch this off.
- **Profile pages.** Profile pages show only the images their owner chooses to show.

### Changed
- **Signing in with HEL.ink.** It no longer overwrites a name you edited here.

## [3.0.0] - 2026-10-15 - PixelHop rebuilt

PixelHop was rebuilt from scratch as one small program.

### Added
- **One program.** The web server, the database and the cleanup job are one file of about 16 MB, with a Docker image and service files included.
- **Privacy.** Location and camera data are removed from photos without changing the picture, after the rotation is applied.
- **Studio.** A studio in the browser for compressing, resizing, cropping, converting, adjusting, blurring, finding colors, reading text and removing backgrounds, all on your own device.
- **Albums and links.** Albums for multi-file uploads, QR codes, Markdown, HTML and BBCode embeds, and readable links such as beach-sunset-x7Kp2Q.
- **Self-destruct and delete links.** Files can disappear after a time from 10 minutes to 90 days, guests get a private delete link, and each device keeps an upload history.
- **API keys.** Keys are stored hashed and shown once, with a one-click ShareX setup and an upload, list, rename and delete API.
- **Storage.** Local disk or any S3-compatible bucket.
- **Admin.** Overview with a 30-day chart, images, users, reports, IP bans and site settings.
- **Import.** Users and images of PixelHop 2 move over, with their passwords.
- **One account with HEL.ink.** Optional sign-in with a HEL.ink account, with sign-up, password resets, Google and two-step sign-in handled there.
- **Server AI tools.** Optional background removal, automatic face blur, magic eraser and HD upscaling, for signed-in people with daily limits.
- **Batch page.** The same steps for many images, then one ZIP or one album.
- **Bulk links.** Copy all links after bulk uploads, on albums and on old result links, plus share buttons and select-all in the gallery.
- **QR code generator.** A generator for any text.
- **Protection.** Cloudflare Turnstile, a Help page, usage and disk space in the admin and redirects for every PixelHop 2 address.

### Security
- **Upload from a link.** It cannot be tricked into reaching internal addresses, including through redirects.
- **Proxies.** Forwarding headers are trusted only from configured proxies.
- **Pages.** Cross-site requests are blocked, scripts are tightly limited and images are served in a sandbox.
- **Accounts.** Passwords use Argon2id, sign-in is rate-limited, session tokens are stored hashed, and a guard stops oversized images that unpack to enormous sizes.

### Changed
- **Studio.** The old server-side Python tools were replaced by tools that run in the browser, so server upscaling and erasing were not offered at first.
- **Database.** The old database was replaced by SQLite.

## [2.3.0] - 2026-01-03 - Readable image links

### Added
- **Image links.** New uploads get a link with a short part of the file name, such as rumah-baru_a3x9K2, so you can tell images apart from the link alone. Long names are cut at word boundaries, and copying many links shows which is which, even for guests.

### Changed
- **Old links.** Every old link keeps working, and only new uploads use the new form.

### Fixed
- **Links.** File names are cleaned better when turned into link text.

## [2.2.0] - 2025-12-18 - Views, reports and account status

### Added
- **Views.** Every image counts its views once per hour per visitor, shows the count and last view date, and galleries can be sorted by most views.
- **Reports.** Visitors can report an image as adult, illegal, a privacy problem or a copyright problem, and admins review a queue of reports.
- **Accounts.** Accounts can be active, locked or suspended, and admins can send a warning that pops up on the member's next sign-in.
- **Expiry.** Images are deleted after 90 days without views.

### Changed
- **Admin.** The gallery statistics are more compact, the image page shows view count and a report button, and the users page manages account status.

### Fixed
- **Admin.** Dialogs now look right in light mode and a dismissed report is disabled in the list.

## [2.1.0] - 2025-12-18 - Two storage providers

### Added
- **Storage.** Thumbnails and originals are kept at two storage providers, with automatic switching if one fails, and the admin shows usage for each.
- **Protection.** A firewall blocks addresses, limits request rates and spots abuse.

### Changed
- **Admin.** The admin has five menus instead of ten and a shared header, with larger gauges and progress bars for storage.

### Fixed
- **Settings and admin.** The member settings page layout and the admin page centering were corrected, and storage statistics show real usage.

## [2.0.0] - 2025-12-17 - New admin panel

### Added
- **Admin.** A new admin panel with a glassmorphism look, a dark and light switch and live server health.

### Changed
- **Admin.** The whole admin was redesigned and made responsive.

## [1.0.0] - 2025-12-16 - First release

### Added
- **Upload.** Image upload in many formats, with sign-in through Google, rate limits and abuse protection.
- **Studio.** Tools to resize, crop, compress and convert images, read text in pictures and remove backgrounds.
