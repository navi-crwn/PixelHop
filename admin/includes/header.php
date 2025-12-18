<?php
/**
 * PixelHop Admin - Shared Header Component
 * Consistent navigation across all admin pages
 * 
 * Usage: 
 * $currentPage = 'dashboard'; // or 'users', 'gallery', 'security', 'settings'
 * include __DIR__ . '/includes/header.php';
 */

$adminMenus = [
    'dashboard' => ['icon' => 'layout-dashboard', 'label' => 'Dashboard', 'url' => '/admin/dashboard.php'],
    'users' => ['icon' => 'users', 'label' => 'Users', 'url' => '/admin/users.php'],
    'gallery' => ['icon' => 'images', 'label' => 'Gallery', 'url' => '/admin/gallery.php'],
    'security' => ['icon' => 'shield', 'label' => 'Security', 'url' => '/admin/abuse.php'],
    'settings' => ['icon' => 'settings', 'label' => 'Settings', 'url' => '/admin/settings.php'],
];

// Submenu for each section
$subMenus = [
    'gallery' => [
        ['icon' => 'images', 'label' => 'All Images', 'url' => '/admin/gallery.php'],
        ['icon' => 'wrench', 'label' => 'Tools Stats', 'url' => '/admin/tools.php'],
    ],
    'security' => [
        ['icon' => 'shield-alert', 'label' => 'Abuse Reports', 'url' => '/admin/abuse.php'],
        ['icon' => 'flame', 'label' => 'Firewall', 'url' => '/admin/firewall.php'],
        ['icon' => 'database', 'label' => 'Storage', 'url' => '/admin/storage.php'],
    ],
    'settings' => [
        ['icon' => 'settings', 'label' => 'General', 'url' => '/admin/settings.php'],
        ['icon' => 'globe', 'label' => 'SEO', 'url' => '/admin/seo.php'],
    ],
];
?>
<!-- Admin Header -->
<header class="admin-header">
    <div class="header-content">
        <div class="logo-section">
            <div class="logo-icon">
                <i data-lucide="shield" class="w-6 h-6"></i>
            </div>
            <div class="logo-text">
                <h1>PixelHop Admin</h1>
                <span><?= ucfirst($currentPage ?? 'Dashboard') ?></span>
            </div>
        </div>
        
        <nav class="nav-main">
            <?php foreach ($adminMenus as $key => $menu): ?>
            <a href="<?= $menu['url'] ?>" class="nav-link <?= ($currentPage ?? '') === $key ? 'active' : '' ?>">
                <i data-lucide="<?= $menu['icon'] ?>" class="w-4 h-4"></i>
                <span><?= $menu['label'] ?></span>
            </a>
            <?php endforeach; ?>
        </nav>

        <div class="header-actions">
            <button class="theme-toggle" onclick="toggleTheme()" title="Toggle Theme">
                <i data-lucide="sun" class="theme-icon-light w-5 h-5"></i>
                <i data-lucide="moon" class="theme-icon-dark w-5 h-5"></i>
            </button>
            <a href="/" class="nav-link site-link">
                <i data-lucide="home" class="w-4 h-4"></i>
                <span>Site</span>
            </a>
        </div>
    </div>
    
    <?php if (isset($subMenus[$currentPage ?? ''])): ?>
    <div class="sub-nav">
        <?php foreach ($subMenus[$currentPage] as $sub): ?>
        <a href="<?= $sub['url'] ?>" class="sub-link <?= ($_SERVER['REQUEST_URI'] === $sub['url']) ? 'active' : '' ?>">
            <i data-lucide="<?= $sub['icon'] ?>" class="w-4 h-4"></i>
            <?= $sub['label'] ?>
        </a>
        <?php endforeach; ?>
    </div>
    <?php endif; ?>
</header>

<!-- Mobile Menu Toggle -->
<button class="mobile-menu-toggle" onclick="toggleMobileMenu()">
    <i data-lucide="menu" class="w-6 h-6"></i>
</button>

<div class="mobile-menu" id="mobileMenu">
    <div class="mobile-menu-header">
        <span>Menu</span>
        <button onclick="toggleMobileMenu()">
            <i data-lucide="x" class="w-5 h-5"></i>
        </button>
    </div>
    <?php foreach ($adminMenus as $key => $menu): ?>
    <a href="<?= $menu['url'] ?>" class="mobile-nav-link <?= ($currentPage ?? '') === $key ? 'active' : '' ?>">
        <i data-lucide="<?= $menu['icon'] ?>" class="w-5 h-5"></i>
        <?= $menu['label'] ?>
    </a>
    <?php endforeach; ?>
    <a href="/" class="mobile-nav-link">
        <i data-lucide="home" class="w-5 h-5"></i>
        Back to Site
    </a>
</div>
