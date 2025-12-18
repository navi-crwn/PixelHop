<?php
/**
 * PixelHop - Admin Gallery
 * View all uploaded images
 */

session_start();
require_once __DIR__ . '/../auth/middleware.php';
require_once __DIR__ . '/../includes/Database.php';
require_once __DIR__ . '/../core/Gatekeeper.php';

if (!isAuthenticated() || !isAdmin()) {
    header('Location: /login.php?error=access_denied');
    exit;
}

$db = Database::getInstance();
$gatekeeper = new Gatekeeper();
$currentUser = getCurrentUser();

// Handle delete action
if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['ajax'])) {
    header('Content-Type: application/json');
    
    if (!validateCsrfToken($_POST['csrf_token'] ?? '')) {
        echo json_encode(['error' => 'Invalid token']);
        exit;
    }
    
    $action = $_POST['action'] ?? '';
    
    if ($action === 'delete_image') {
        $imageId = $_POST['image_id'] ?? '';
        $imagesFile = __DIR__ . '/../data/images.json';
        
        if (file_exists($imagesFile)) {
            $images = json_decode(file_get_contents($imagesFile), true) ?: [];
            if (isset($images[$imageId])) {
                unset($images[$imageId]);
                file_put_contents($imagesFile, json_encode($images, JSON_PRETTY_PRINT));
                echo json_encode(['success' => true]);
                exit;
            }
        }
        echo json_encode(['error' => 'Image not found']);
    }
    exit;
}

// Pagination
$page = max(1, (int)($_GET['page'] ?? 1));
$perPage = 20;
$offset = ($page - 1) * $perPage;

// Search/Filter
$search = $_GET['search'] ?? '';

// Load images from JSON
$imagesFile = __DIR__ . '/../data/images.json';
$allImages = [];
if (file_exists($imagesFile)) {
    $allImages = json_decode(file_get_contents($imagesFile), true) ?: [];
}

// Get all users for lookup
$usersStmt = $db->query("SELECT id, email, account_type FROM users");
$usersMap = [];
while ($row = $usersStmt->fetch(PDO::FETCH_ASSOC)) {
    $usersMap[$row['id']] = $row;
}

// Process and filter images
$processedImages = [];
foreach ($allImages as $id => $img) {
    $img['id'] = $id;
    
    $userId = $img['user_id'] ?? null;
    if ($userId && isset($usersMap[$userId])) {
        $img['user_email'] = $usersMap[$userId]['email'];
        $img['is_guest'] = false;
    } else {
        $img['user_email'] = 'Guest';
        $img['is_guest'] = true;
    }
    
    if ($search) {
        $matchFilename = stripos($img['filename'] ?? '', $search) !== false;
        $matchId = stripos($id, $search) !== false;
        $matchEmail = stripos($img['user_email'] ?? '', $search) !== false;
        if (!$matchFilename && !$matchId && !$matchEmail) continue;
    }
    
    $processedImages[] = $img;
}

// Sort by date (newest first)
usort($processedImages, fn($a, $b) => ($b['created_at'] ?? 0) - ($a['created_at'] ?? 0));

$totalImages = count($processedImages);
$totalPages = ceil($totalImages / $perPage);
$images = array_slice($processedImages, $offset, $perPage);

// Calculate stats
$totalSize = array_sum(array_column($allImages, 'size'));
$guestCount = count(array_filter($allImages, fn($i) => empty($i['user_id'])));
$memberCount = count($allImages) - $guestCount;

function formatBytes($bytes) {
    if ($bytes >= 1073741824) return round($bytes / 1073741824, 2) . ' GB';
    if ($bytes >= 1048576) return round($bytes / 1048576, 2) . ' MB';
    if ($bytes >= 1024) return round($bytes / 1024, 2) . ' KB';
    return $bytes . ' B';
}

$csrfToken = generateCsrfToken();
$currentPage = 'gallery';
?>
<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Gallery - Admin - PixelHop</title>
    <link rel="icon" type="image/svg+xml" href="/assets/img/logo.svg">
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <script src="https://unpkg.com/lucide@latest/dist/umd/lucide.js"></script>
    <link rel="stylesheet" href="/admin/includes/admin-styles.css">
    <script src="/admin/includes/admin-scripts.js"></script>
</head>
<body>
    <div class="admin-wrapper">
        <div class="admin-container">
            <?php include __DIR__ . '/includes/header.php'; ?>
            
            <div class="admin-content">
                <!-- Stats Grid -->
                <div class="grid-4 mb-6">
                    <div class="stat-card">
                        <div class="stat-label">
                            <i data-lucide="images" class="w-4 h-4 text-cyan"></i>
                            Total Images
                        </div>
                        <div class="stat-value"><?= number_format(count($allImages)) ?></div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">
                            <i data-lucide="hard-drive" class="w-4 h-4 text-purple"></i>
                            Total Size
                        </div>
                        <div class="stat-value"><?= formatBytes($totalSize) ?></div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">
                            <i data-lucide="user" class="w-4 h-4 text-green"></i>
                            Member Uploads
                        </div>
                        <div class="stat-value"><?= number_format($memberCount) ?></div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">
                            <i data-lucide="user-x" class="w-4 h-4 text-yellow"></i>
                            Guest Uploads
                        </div>
                        <div class="stat-value"><?= number_format($guestCount) ?></div>
                    </div>
                </div>

                <!-- Search -->
                <div class="card mb-6">
                    <form method="GET" class="flex gap-4" style="flex-wrap: wrap;">
                        <input type="text" name="search" class="form-input" placeholder="Search by ID, filename, or email..." 
                               value="<?= htmlspecialchars($search) ?>" style="flex: 1; min-width: 200px;">
                        <button type="submit" class="btn btn-primary">
                            <i data-lucide="search" class="w-4 h-4"></i>
                            Search
                        </button>
                        <?php if ($search): ?>
                        <a href="/admin/gallery.php" class="btn btn-secondary">Clear</a>
                        <?php endif; ?>
                    </form>
                </div>

                <!-- Gallery Grid -->
                <div class="card">
                    <div class="card-header">
                        <div class="card-title">
                            <i data-lucide="grid-3x3" class="w-5 h-5"></i>
                            Images (Page <?= $page ?> of <?= max(1, $totalPages) ?>)
                        </div>
                    </div>
                    
                    <div class="gallery-grid">
                        <?php foreach ($images as $img): ?>
                        <div class="gallery-item" data-id="<?= htmlspecialchars($img['id']) ?>">
                            <div class="gallery-thumb">
                                <img src="<?= htmlspecialchars($img['urls']['thumb'] ?? $img['urls']['medium'] ?? '') ?>" 
                                     alt="" loading="lazy" onerror="this.src='/assets/img/placeholder.png'">
                            </div>
                            <div class="gallery-info">
                                <div class="gallery-id"><?= htmlspecialchars($img['id']) ?></div>
                                <div class="gallery-meta">
                                    <?= formatBytes($img['size'] ?? 0) ?> • <?= date('M j', $img['created_at'] ?? 0) ?>
                                </div>
                                <div class="gallery-user text-muted"><?= htmlspecialchars($img['user_email']) ?></div>
                            </div>
                            <div class="gallery-actions">
                                <a href="/<?= htmlspecialchars($img['id']) ?>" target="_blank" class="btn-icon" title="View">
                                    <i data-lucide="external-link" class="w-4 h-4"></i>
                                </a>
                                <button class="btn-icon btn-delete" title="Delete">
                                    <i data-lucide="trash-2" class="w-4 h-4"></i>
                                </button>
                            </div>
                        </div>
                        <?php endforeach; ?>
                        
                        <?php if (empty($images)): ?>
                        <div class="no-results">
                            <i data-lucide="image-off" class="w-12 h-12 text-muted"></i>
                            <p class="text-muted">No images found</p>
                        </div>
                        <?php endif; ?>
                    </div>

                    <!-- Pagination -->
                    <?php if ($totalPages > 1): ?>
                    <div class="pagination">
                        <?php if ($page > 1): ?>
                        <a href="?page=<?= $page - 1 ?><?= $search ? '&search=' . urlencode($search) : '' ?>" class="btn btn-secondary">← Previous</a>
                        <?php endif; ?>
                        <span class="text-muted">Page <?= $page ?> of <?= $totalPages ?></span>
                        <?php if ($page < $totalPages): ?>
                        <a href="?page=<?= $page + 1 ?><?= $search ? '&search=' . urlencode($search) : '' ?>" class="btn btn-secondary">Next →</a>
                        <?php endif; ?>
                    </div>
                    <?php endif; ?>
                </div>
            </div>

            <footer class="admin-footer">
                <span>© 2025 PixelHop • Admin Panel v2.0</span>
                <div class="footer-links">
                    <a href="/dashboard.php">My Account</a>
                    <a href="/tools">Tools</a>
                    <a href="/auth/logout.php" class="text-red">Logout</a>
                </div>
            </footer>
        </div>
    </div>

    <style>
        .gallery-grid {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
            gap: 16px;
            padding: 16px 0;
        }
        .gallery-item {
            background: var(--border-color);
            border-radius: 12px;
            overflow: hidden;
            transition: all 0.2s;
        }
        .gallery-item:hover {
            transform: translateY(-2px);
            box-shadow: 0 8px 25px rgba(0,0,0,0.3);
        }
        .gallery-thumb {
            aspect-ratio: 1;
            overflow: hidden;
            background: rgba(0,0,0,0.2);
        }
        .gallery-thumb img {
            width: 100%;
            height: 100%;
            object-fit: cover;
        }
        .gallery-info {
            padding: 12px;
        }
        .gallery-id {
            font-weight: 600;
            font-size: 14px;
            color: var(--text-primary);
        }
        .gallery-meta {
            font-size: 12px;
            color: var(--text-muted);
            margin-top: 2px;
        }
        .gallery-user {
            font-size: 11px;
            margin-top: 4px;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }
        .gallery-actions {
            display: flex;
            gap: 6px;
            padding: 0 12px 12px;
        }
        .btn-icon {
            width: 32px;
            height: 32px;
            border-radius: 8px;
            border: 1px solid var(--border-color);
            background: transparent;
            color: var(--text-secondary);
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.2s;
            text-decoration: none;
        }
        .btn-icon:hover { background: var(--border-color-hover); color: var(--text-primary); }
        .btn-delete:hover { background: rgba(239, 68, 68, 0.15); color: var(--accent-red); }
        .no-results {
            grid-column: 1 / -1;
            text-align: center;
            padding: 60px 20px;
        }
        .pagination {
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 16px;
            padding: 16px;
            border-top: 1px solid var(--border-color);
        }
    </style>

    <script>
        document.addEventListener('DOMContentLoaded', function() {
            lucide.createIcons();
            
            const csrf = '<?= $csrfToken ?>';
            
            document.querySelectorAll('.btn-delete').forEach(btn => {
                btn.onclick = async function() {
                    if (!confirm('Are you sure you want to delete this image?')) return;
                    
                    const item = this.closest('.gallery-item');
                    const imageId = item.dataset.id;
                    
                    const fd = new FormData();
                    fd.append('ajax', '1');
                    fd.append('action', 'delete_image');
                    fd.append('csrf_token', csrf);
                    fd.append('image_id', imageId);
                    
                    const res = await fetch('/admin/gallery.php', { method: 'POST', body: fd });
                    const data = await res.json();
                    
                    if (data.success) {
                        item.remove();
                    } else {
                        alert('Failed to delete: ' + (data.error || 'Unknown error'));
                    }
                };
            });
        });
    </script>
</body>
</html>
