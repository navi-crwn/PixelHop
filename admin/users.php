<?php
/**
 * PixelHop - Admin Users Management
 * Clean responsive design
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

// Handle AJAX requests
if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['ajax'])) {
    header('Content-Type: application/json');

    if (!validateCsrfToken($_POST['csrf_token'] ?? '')) {
        echo json_encode(['error' => 'Invalid token']);
        exit;
    }

    $action = $_POST['action'] ?? '';

    switch ($action) {
        case 'toggle_user':
            $userId = (int) ($_POST['user_id'] ?? 0);
            $block = (int) ($_POST['block'] ?? 0);
            if ($userId > 0 && $userId !== $currentUser['id']) {
                $stmt = $db->prepare("UPDATE users SET is_blocked = ? WHERE id = ?");
                $stmt->execute([$block, $userId]);
                echo json_encode(['success' => true]);
            } else {
                echo json_encode(['error' => 'Cannot modify this user']);
            }
            break;

        case 'change_role':
            $userId = (int) ($_POST['user_id'] ?? 0);
            $role = $_POST['role'] ?? 'user';
            if ($userId > 0 && $userId !== $currentUser['id'] && in_array($role, ['user', 'admin'])) {
                $stmt = $db->prepare("UPDATE users SET role = ? WHERE id = ?");
                $stmt->execute([$role, $userId]);
                echo json_encode(['success' => true]);
            } else {
                echo json_encode(['error' => 'Cannot modify this user']);
            }
            break;

        case 'change_type':
            $userId = (int) ($_POST['user_id'] ?? 0);
            $type = $_POST['type'] ?? 'free';
            if ($userId > 0 && in_array($type, ['free', 'premium'])) {
                $stmt = $db->prepare("UPDATE users SET account_type = ? WHERE id = ?");
                $stmt->execute([$type, $userId]);
                echo json_encode(['success' => true]);
            } else {
                echo json_encode(['error' => 'Invalid account type']);
            }
            break;

        case 'delete_user':
            $userId = (int) ($_POST['user_id'] ?? 0);
            if ($userId > 0 && $userId !== $currentUser['id']) {
                $stmt = $db->prepare("DELETE FROM users WHERE id = ?");
                $stmt->execute([$userId]);
                echo json_encode(['success' => true]);
            } else {
                echo json_encode(['error' => 'Cannot delete this user']);
            }
            break;

        default:
            echo json_encode(['error' => 'Unknown action']);
    }
    exit;
}

// Get users with pagination
$page = max(1, (int) ($_GET['page'] ?? 1));
$perPage = 20;
$offset = ($page - 1) * $perPage;

$totalUsers = $db->query("SELECT COUNT(*) FROM users")->fetchColumn();
$totalPages = ceil($totalUsers / $perPage);

$users = $db->query("SELECT * FROM users ORDER BY created_at DESC LIMIT $perPage OFFSET $offset")->fetchAll(PDO::FETCH_ASSOC);

// Stats
$premiumCount = $db->query("SELECT COUNT(*) FROM users WHERE account_type = 'premium'")->fetchColumn();
$adminCount = $db->query("SELECT COUNT(*) FROM users WHERE role = 'admin'")->fetchColumn();
$blockedCount = $db->query("SELECT COUNT(*) FROM users WHERE is_blocked = 1")->fetchColumn();

function formatBytes($bytes) {
    if ($bytes >= 1073741824) return round($bytes / 1073741824, 2) . ' GB';
    if ($bytes >= 1048576) return round($bytes / 1048576, 2) . ' MB';
    if ($bytes >= 1024) return round($bytes / 1024, 2) . ' KB';
    return $bytes . ' B';
}

$csrfToken = generateCsrfToken();
$currentPage = 'users';
?>
<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Users - Admin - PixelHop</title>
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
                            <i data-lucide="users" class="w-4 h-4 text-cyan"></i>
                            Total Users
                        </div>
                        <div class="stat-value"><?= number_format($totalUsers) ?></div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">
                            <i data-lucide="crown" class="w-4 h-4 text-yellow"></i>
                            Premium
                        </div>
                        <div class="stat-value"><?= number_format($premiumCount) ?></div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">
                            <i data-lucide="shield" class="w-4 h-4 text-purple"></i>
                            Admins
                        </div>
                        <div class="stat-value"><?= number_format($adminCount) ?></div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-label">
                            <i data-lucide="ban" class="w-4 h-4 text-red"></i>
                            Blocked
                        </div>
                        <div class="stat-value"><?= number_format($blockedCount) ?></div>
                    </div>
                </div>

                <!-- Users Table -->
                <div class="card">
                    <div class="card-header">
                        <div class="card-title">
                            <i data-lucide="list" class="w-5 h-5"></i>
                            User List (Page <?= $page ?> of <?= max(1, $totalPages) ?>)
                        </div>
                    </div>
                    <div class="table-wrapper">
                        <table class="table">
                            <thead>
                                <tr>
                                    <th>User</th>
                                    <th>Type</th>
                                    <th>Role</th>
                                    <th>Storage</th>
                                    <th>Joined</th>
                                    <th>Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                <?php foreach ($users as $user): ?>
                                <tr data-id="<?= $user['id'] ?>" class="<?= $user['is_blocked'] ? 'blocked-row' : '' ?>">
                                    <td>
                                        <div style="display: flex; align-items: center; gap: 10px;">
                                            <div class="user-avatar">
                                                <?= strtoupper(substr($user['email'], 0, 1)) ?>
                                            </div>
                                            <div>
                                                <div style="font-weight: 500;"><?= htmlspecialchars($user['name'] ?? 'No name') ?></div>
                                                <div class="text-muted" style="font-size: 12px;"><?= htmlspecialchars($user['email']) ?></div>
                                            </div>
                                        </div>
                                    </td>
                                    <td>
                                        <span class="badge <?= $user['account_type'] === 'premium' ? 'badge-yellow' : 'badge-gray' ?>">
                                            <?= ucfirst($user['account_type'] ?? 'free') ?>
                                        </span>
                                    </td>
                                    <td>
                                        <span class="badge <?= $user['role'] === 'admin' ? 'badge-purple' : 'badge-gray' ?>">
                                            <?= ucfirst($user['role'] ?? 'user') ?>
                                        </span>
                                    </td>
                                    <td><?= formatBytes($user['storage_used'] ?? 0) ?></td>
                                    <td><?= date('M j, Y', strtotime($user['created_at'])) ?></td>
                                    <td>
                                        <?php if ($user['id'] !== $currentUser['id']): ?>
                                        <div class="action-btns">
                                            <button class="btn-icon btn-toggle" title="<?= $user['is_blocked'] ? 'Unblock' : 'Block' ?>">
                                                <i data-lucide="<?= $user['is_blocked'] ? 'unlock' : 'lock' ?>" class="w-4 h-4"></i>
                                            </button>
                                            <button class="btn-icon btn-delete" title="Delete">
                                                <i data-lucide="trash-2" class="w-4 h-4"></i>
                                            </button>
                                        </div>
                                        <?php else: ?>
                                        <span class="text-muted">You</span>
                                        <?php endif; ?>
                                    </td>
                                </tr>
                                <?php endforeach; ?>
                            </tbody>
                        </table>
                    </div>

                    <!-- Pagination -->
                    <?php if ($totalPages > 1): ?>
                    <div class="pagination">
                        <?php if ($page > 1): ?>
                        <a href="?page=<?= $page - 1 ?>" class="btn btn-secondary">← Previous</a>
                        <?php endif; ?>
                        <span class="text-muted">Page <?= $page ?> of <?= $totalPages ?></span>
                        <?php if ($page < $totalPages): ?>
                        <a href="?page=<?= $page + 1 ?>" class="btn btn-secondary">Next →</a>
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
        .user-avatar {
            width: 36px;
            height: 36px;
            border-radius: 50%;
            background: linear-gradient(135deg, var(--accent-cyan), var(--accent-purple));
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 600;
            font-size: 14px;
            color: white;
        }
        .badge {
            padding: 4px 10px;
            border-radius: 6px;
            font-size: 12px;
            font-weight: 500;
        }
        .badge-gray { background: var(--border-color); color: var(--text-secondary); }
        .badge-yellow { background: rgba(234, 179, 8, 0.15); color: var(--accent-yellow); }
        .badge-purple { background: rgba(168, 85, 247, 0.15); color: var(--accent-purple); }
        .blocked-row { opacity: 0.5; }
        .action-btns { display: flex; gap: 6px; }
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
        }
        .btn-icon:hover { background: var(--border-color-hover); color: var(--text-primary); }
        .btn-delete:hover { background: rgba(239, 68, 68, 0.15); color: var(--accent-red); }
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
            
            async function api(action, data = {}) {
                const fd = new FormData();
                fd.append('ajax', '1');
                fd.append('action', action);
                fd.append('csrf_token', csrf);
                for (const k in data) fd.append(k, data[k]);
                const res = await fetch('/admin/users.php', { method: 'POST', body: fd });
                return res.json();
            }
            
            document.querySelectorAll('.btn-toggle').forEach(btn => {
                btn.onclick = async function() {
                    const row = this.closest('tr');
                    const userId = row.dataset.id;
                    const isBlocked = row.classList.contains('blocked-row');
                    const r = await api('toggle_user', { user_id: userId, block: isBlocked ? 0 : 1 });
                    if (r.success) location.reload();
                };
            });
            
            document.querySelectorAll('.btn-delete').forEach(btn => {
                btn.onclick = async function() {
                    if (!confirm('Are you sure you want to delete this user?')) return;
                    const row = this.closest('tr');
                    const userId = row.dataset.id;
                    const r = await api('delete_user', { user_id: userId });
                    if (r.success) row.remove();
                };
            });
        });
    </script>
</body>
</html>
