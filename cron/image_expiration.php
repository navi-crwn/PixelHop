<?php
/**
 * PixelHop - Image Expiration Cron
 * 
 * Manages automatic deletion of inactive public images:
 * - Public uploads (guest, no user_id) not viewed for 60 days → marked for deletion
 * - Marked images not viewed for additional 30 days (90 days total) → deleted
 * 
 * Run: crontab -e
 * 0 2 * * * php /var/www/pichost/cron/image_expiration.php >> /var/log/pichost/expiration.log 2>&1
 */

// CLI only
if (php_sapi_name() !== 'cli') {
    die('CLI only');
}

define('ROOT_PATH', dirname(__DIR__));

// Load config
$config = require ROOT_PATH . '/config/s3.php';

// Load R2 storage manager for deletion
require_once ROOT_PATH . '/includes/R2StorageManager.php';
$r2 = new R2StorageManager($config);

$dataFile = ROOT_PATH . '/data/images.json';
$logFile = ROOT_PATH . '/data/expiration_log.json';

echo "[" . date('Y-m-d H:i:s') . "] Starting image expiration check...\n";

// Load images
if (!file_exists($dataFile)) {
    die("Error: images.json not found\n");
}

$images = json_decode(file_get_contents($dataFile), true);
if (!is_array($images)) {
    die("Error: Invalid images.json format\n");
}

$now = time();
$sixtyDays = 60 * 24 * 60 * 60; // 60 days in seconds
$thirtyDays = 30 * 24 * 60 * 60; // 30 days in seconds

$stats = [
    'checked' => 0,
    'skipped_user_owned' => 0,
    'marked_for_deletion' => 0,
    'deleted' => 0,
    'deletion_errors' => 0,
    'still_active' => 0
];

$toDelete = [];
$modified = false;

foreach ($images as $imageId => $image) {
    $stats['checked']++;
    
    // Skip user-owned images (registered users)
    if (!empty($image['user_id'])) {
        $stats['skipped_user_owned']++;
        continue;
    }
    
    // Get the last viewed timestamp (or fall back to created_at)
    $lastViewed = $image['last_viewed_at'] ?? $image['created_at'] ?? 0;
    $daysSinceView = ($now - $lastViewed) / (24 * 60 * 60);
    
    // Check if already marked for deletion
    if (isset($image['marked_for_deletion'])) {
        $markedAt = $image['marked_for_deletion'];
        $daysSinceMarked = ($now - $markedAt) / (24 * 60 * 60);
        
        // If 30 more days have passed since marking (90 days total without view), delete
        if ($daysSinceMarked >= 30) {
            $toDelete[] = $imageId;
            echo "  [DELETE] {$imageId} - No views for 90+ days (marked " . round($daysSinceMarked) . " days ago)\n";
        } else {
            echo "  [PENDING] {$imageId} - Marked for deletion, " . round($daysSinceMarked, 1) . " days ago (will delete in " . round(30 - $daysSinceMarked) . " days)\n";
        }
    } else {
        // Not yet marked - check if it's been 60 days without a view
        if ($daysSinceView >= 60) {
            // Mark for deletion
            $images[$imageId]['marked_for_deletion'] = $now;
            $modified = true;
            $stats['marked_for_deletion']++;
            echo "  [MARKED] {$imageId} - No views for " . round($daysSinceView) . " days, marked for deletion (will delete in 30 days)\n";
        } else {
            $stats['still_active']++;
        }
    }
}

// Delete marked images that have exceeded the grace period
foreach ($toDelete as $imageId) {
    $image = $images[$imageId] ?? null;
    if (!$image) continue;
    
    $deleteSuccess = true;
    $s3Keys = $image['s3_keys'] ?? [];
    $storageProviders = $image['storage_providers'] ?? [];
    
    // Delete from storage
    foreach ($s3Keys as $sizeType => $s3Key) {
        $provider = $storageProviders[$sizeType] ?? 'contabo';
        
        try {
            if ($provider === 'r2') {
                $result = $r2->deleteFromR2($s3Key);
            } else {
                $result = $r2->deleteFromContabo($s3Key);
            }
            
            if ($result['success']) {
                echo "    Deleted {$sizeType} from {$provider}: {$s3Key}\n";
            } else {
                echo "    Warning: Failed to delete {$sizeType} from {$provider}: {$s3Key} - " . ($result['error'] ?? 'Unknown error') . "\n";
            }
        } catch (Exception $e) {
            echo "    Error deleting {$sizeType}: " . $e->getMessage() . "\n";
            $deleteSuccess = false;
        }
    }
    
    // Remove from images.json
    if ($deleteSuccess) {
        unset($images[$imageId]);
        $modified = true;
        $stats['deleted']++;
        echo "  [REMOVED] {$imageId} from database\n";
    } else {
        $stats['deletion_errors']++;
    }
}

// Save modified images.json
if ($modified) {
    $result = file_put_contents($dataFile, json_encode($images, JSON_PRETTY_PRINT | JSON_UNESCAPED_SLASHES), LOCK_EX);
    if ($result) {
        echo "\nSaved changes to images.json\n";
    } else {
        echo "\nError: Failed to save images.json\n";
    }
}

// Save log
$log = [
    'timestamp' => date('Y-m-d H:i:s'),
    'stats' => $stats
];

$logs = [];
if (file_exists($logFile)) {
    $logs = json_decode(file_get_contents($logFile), true) ?: [];
}
$logs[] = $log;

// Keep only last 30 days of logs
$logs = array_slice($logs, -30);
file_put_contents($logFile, json_encode($logs, JSON_PRETTY_PRINT), LOCK_EX);

echo "\n=== Summary ===\n";
echo "Checked: {$stats['checked']}\n";
echo "Skipped (user-owned): {$stats['skipped_user_owned']}\n";
echo "Still active: {$stats['still_active']}\n";
echo "Newly marked for deletion: {$stats['marked_for_deletion']}\n";
echo "Deleted: {$stats['deleted']}\n";
echo "Deletion errors: {$stats['deletion_errors']}\n";
echo "[" . date('Y-m-d H:i:s') . "] Done.\n";
