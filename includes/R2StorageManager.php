<?php
/**
 * PixelHop - Cloudflare R2 Storage Manager
 * Hybrid storage with Contabo S3 fallback
 * 
 * Features:
 * - Hard limit to stay within free tier (9.5GB default)
 * - Auto fallback to Contabo when limit reached
 * - Usage tracking in database
 * - Rate limiting for R2 operations
 * - S3-compatible API
 */

require_once __DIR__ . '/R2RateLimiter.php';

class R2StorageManager
{
    // Free tier limit with buffer (9.5GB to be safe)
    private const FREE_TIER_LIMIT = 9.5 * 1024 * 1024 * 1024; // 9.5GB in bytes
    
    // Warning threshold (8GB)
    private const WARNING_THRESHOLD = 8 * 1024 * 1024 * 1024;
    
    private array $r2Config;
    private array $contaboConfig;
    private ?PDO $db = null;
    private bool $r2Enabled = false;
    private ?R2RateLimiter $rateLimiter = null;
    
    public function __construct(array $config)
    {
        $this->contaboConfig = $config['s3'] ?? [];
        $this->r2Config = $config['r2'] ?? [];
        $this->r2Enabled = !empty($this->r2Config['enabled']) && 
                           !empty($this->r2Config['access_key']) && 
                           !empty($this->r2Config['bucket']);
        
        // Initialize database connection
        $this->initDatabase();
        
        // Initialize rate limiter for R2
        if ($this->r2Enabled) {
            $this->rateLimiter = new R2RateLimiter();
        }
    }
    
    /**
     * Initialize database connection
     */
    private function initDatabase(): void
    {
        try {
            require_once __DIR__ . '/Database.php';
            $this->db = Database::getInstance();
        } catch (Exception $e) {
            error_log('R2StorageManager: Database connection failed - ' . $e->getMessage());
        }
    }
    
    /**
     * Check if R2 is enabled and configured
     */
    public function isR2Enabled(): bool
    {
        return $this->r2Enabled;
    }
    
    /**
     * Get current R2 storage usage from database
     */
    public function getR2Usage(): array
    {
        $usage = 0;
        $fileCount = 0;
        
        if ($this->db) {
            try {
                // Get from storage_stats table
                $stmt = $this->db->prepare("SELECT total_bytes, file_count FROM storage_stats WHERE provider = 'r2' LIMIT 1");
                $stmt->execute();
                $result = $stmt->fetch(PDO::FETCH_ASSOC);
                
                if ($result) {
                    $usage = (int) $result['total_bytes'];
                    $fileCount = (int) $result['file_count'];
                }
            } catch (PDOException $e) {
                error_log('R2StorageManager: Failed to get usage - ' . $e->getMessage());
            }
        }
        
        $limit = self::FREE_TIER_LIMIT;
        $percentage = $limit > 0 ? round(($usage / $limit) * 100, 2) : 0;
        
        return [
            'used_bytes' => $usage,
            'used_human' => $this->formatBytes($usage),
            'limit_bytes' => $limit,
            'limit_human' => $this->formatBytes($limit),
            'available_bytes' => max(0, $limit - $usage),
            'available_human' => $this->formatBytes(max(0, $limit - $usage)),
            'percentage' => $percentage,
            'file_count' => $fileCount,
            'is_warning' => $usage >= self::WARNING_THRESHOLD,
            'is_full' => $usage >= $limit,
        ];
    }
    
    /**
     * Check if we can upload to R2 (within free tier limit + rate limit)
     */
    public function canUploadToR2(int $fileSize): bool
    {
        if (!$this->r2Enabled) {
            return false;
        }
        
        // Check storage limit
        $usage = $this->getR2Usage();
        if (($usage['used_bytes'] + $fileSize) >= self::FREE_TIER_LIMIT) {
            return false;
        }
        
        // Check rate limit (Class A operations)
        if ($this->rateLimiter && !$this->rateLimiter->canPerformClassA()) {
            error_log('R2StorageManager: Rate limit reached, falling back to Contabo');
            return false;
        }
        
        return true;
    }
    
    /**
     * Determine best storage for file based on type and R2 availability
     * 
     * Strategy:
     * - Thumbnails & Medium → R2 (if space available)
     * - Large & Original → Contabo (unlimited)
     */
    public function determineStorage(string $sizeType, int $fileSize): string
    {
        // Original and large always go to Contabo
        if (in_array($sizeType, ['original', 'large'])) {
            return 'contabo';
        }
        
        // Thumbnails and medium go to R2 if possible
        if (in_array($sizeType, ['thumb', 'medium'])) {
            if ($this->canUploadToR2($fileSize)) {
                return 'r2';
            }
        }
        
        // Fallback to Contabo
        return 'contabo';
    }
    
    /**
     * Upload file to appropriate storage
     * Returns storage provider used and URL
     */
    public function upload(string $filepath, string $key, string $contentType, string $sizeType = 'original'): array
    {
        $fileSize = filesize($filepath);
        $provider = $this->determineStorage($sizeType, $fileSize);
        
        if ($provider === 'r2' && $this->r2Enabled) {
            $result = $this->uploadToR2($filepath, $key, $contentType);
            
            if ($result['success']) {
                // Track R2 usage
                $this->trackUsage('r2', $fileSize, 1);
                
                // Record rate limit operation
                if ($this->rateLimiter) {
                    $this->rateLimiter->recordClassA('PUT', $key, $fileSize);
                }
                
                return [
                    'success' => true,
                    'provider' => 'r2',
                    'url' => $this->getR2PublicUrl($key),
                    'key' => $key,
                ];
            }
            
            // R2 failed, fallback to Contabo
            error_log('R2 upload failed, falling back to Contabo: ' . ($result['error'] ?? 'unknown'));
            $provider = 'contabo';
        }
        
        // Upload to Contabo
        $result = $this->uploadToContabo($filepath, $key, $contentType);
        
        if ($result['success']) {
            $this->trackUsage('contabo', $fileSize, 1);
            
            return [
                'success' => true,
                'provider' => 'contabo',
                'url' => $this->getContaboPublicUrl($key),
                'key' => $key,
            ];
        }
        
        return [
            'success' => false,
            'error' => $result['error'] ?? 'Upload failed',
            'provider' => null,
        ];
    }
    
    /**
     * Upload to Cloudflare R2
     */
    private function uploadToR2(string $filepath, string $key, string $contentType): array
    {
        return $this->uploadToS3(
            $filepath,
            $key,
            $contentType,
            $this->r2Config['endpoint'],
            $this->r2Config['bucket'],
            $this->r2Config['access_key'],
            $this->r2Config['secret_key'],
            $this->r2Config['region'] ?? 'auto'
        );
    }
    
    /**
     * Upload to Contabo S3
     */
    private function uploadToContabo(string $filepath, string $key, string $contentType): array
    {
        return $this->uploadToS3(
            $filepath,
            $key,
            $contentType,
            $this->contaboConfig['endpoint'],
            $this->contaboConfig['bucket'],
            $this->contaboConfig['access_key'],
            $this->contaboConfig['secret_key'],
            $this->contaboConfig['region'] ?? 'default'
        );
    }
    
    /**
     * Generic S3-compatible upload
     */
    private function uploadToS3(
        string $filepath,
        string $key,
        string $contentType,
        string $endpoint,
        string $bucket,
        string $accessKey,
        string $secretKey,
        string $region
    ): array {
        if (!file_exists($filepath)) {
            return ['success' => false, 'error' => 'File not found'];
        }
        
        $fileContent = file_get_contents($filepath);
        $contentLength = strlen($fileContent);
        $payloadHash = hash('sha256', $fileContent);
        
        $parsedUrl = parse_url($endpoint);
        $host = $parsedUrl['host'];
        
        $url = "{$endpoint}/{$bucket}/{$key}";
        
        $longDate = gmdate('Ymd\THis\Z');
        $shortDate = gmdate('Ymd');
        
        $canonicalUri = '/' . $bucket . '/' . str_replace('%2F', '/', rawurlencode($key));
        
        $headers = [
            'content-length' => $contentLength,
            'content-type' => $contentType,
            'host' => $host,
            'x-amz-acl' => 'public-read',
            'x-amz-content-sha256' => $payloadHash,
            'x-amz-date' => $longDate,
        ];
        
        ksort($headers);
        $canonicalHeaders = '';
        $signedHeaders = [];
        foreach ($headers as $k => $v) {
            $canonicalHeaders .= strtolower($k) . ':' . trim($v) . "\n";
            $signedHeaders[] = strtolower($k);
        }
        $signedHeadersStr = implode(';', $signedHeaders);
        
        $canonicalRequest = "PUT\n" .
            $canonicalUri . "\n" .
            "\n" .
            $canonicalHeaders . "\n" .
            $signedHeadersStr . "\n" .
            $payloadHash;
        
        $algorithm = 'AWS4-HMAC-SHA256';
        $credentialScope = "{$shortDate}/{$region}/s3/aws4_request";
        $stringToSign = "{$algorithm}\n{$longDate}\n{$credentialScope}\n" . hash('sha256', $canonicalRequest);
        
        $kDate = hash_hmac('sha256', $shortDate, 'AWS4' . $secretKey, true);
        $kRegion = hash_hmac('sha256', $region, $kDate, true);
        $kService = hash_hmac('sha256', 's3', $kRegion, true);
        $kSigning = hash_hmac('sha256', 'aws4_request', $kService, true);
        $signature = hash_hmac('sha256', $stringToSign, $kSigning);
        
        $authorization = "{$algorithm} Credential={$accessKey}/{$credentialScope}, SignedHeaders={$signedHeadersStr}, Signature={$signature}";
        
        $ch = curl_init();
        curl_setopt_array($ch, [
            CURLOPT_URL => $url,
            CURLOPT_CUSTOMREQUEST => 'PUT',
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_POSTFIELDS => $fileContent,
            CURLOPT_HTTPHEADER => [
                "Authorization: {$authorization}",
                "Content-Type: {$contentType}",
                "Content-Length: {$contentLength}",
                "Host: {$host}",
                "x-amz-acl: public-read",
                "x-amz-content-sha256: {$payloadHash}",
                "x-amz-date: {$longDate}",
            ],
            CURLOPT_SSL_VERIFYPEER => true,
            CURLOPT_TIMEOUT => 120,
            CURLOPT_CONNECTTIMEOUT => 30,
        ]);
        
        $response = curl_exec($ch);
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        $error = curl_error($ch);
        curl_close($ch);
        
        if ($error) {
            return ['success' => false, 'error' => "CURL error: {$error}", 'http_code' => 0];
        }
        
        if ($httpCode < 200 || $httpCode >= 300) {
            return ['success' => false, 'error' => "HTTP {$httpCode}: {$response}", 'http_code' => $httpCode];
        }
        
        return ['success' => true, 'http_code' => $httpCode];
    }
    
    /**
     * Track storage usage in database
     */
    private function trackUsage(string $provider, int $bytes, int $fileCount): void
    {
        if (!$this->db) {
            return;
        }
        
        try {
            // Upsert storage stats
            $stmt = $this->db->prepare("
                INSERT INTO storage_stats (provider, total_bytes, file_count, updated_at)
                VALUES (?, ?, ?, NOW())
                ON DUPLICATE KEY UPDATE 
                    total_bytes = total_bytes + VALUES(total_bytes),
                    file_count = file_count + VALUES(file_count),
                    updated_at = NOW()
            ");
            $stmt->execute([$provider, $bytes, $fileCount]);
        } catch (PDOException $e) {
            error_log('R2StorageManager: Failed to track usage - ' . $e->getMessage());
        }
    }
    
    /**
     * Reduce usage tracking (for deletions)
     */
    public function reduceUsage(string $provider, int $bytes, int $fileCount = 1): void
    {
        if (!$this->db) {
            return;
        }
        
        try {
            $stmt = $this->db->prepare("
                UPDATE storage_stats 
                SET total_bytes = GREATEST(0, total_bytes - ?),
                    file_count = GREATEST(0, file_count - ?),
                    updated_at = NOW()
                WHERE provider = ?
            ");
            $stmt->execute([$bytes, $fileCount, $provider]);
        } catch (PDOException $e) {
            error_log('R2StorageManager: Failed to reduce usage - ' . $e->getMessage());
        }
    }
    
    /**
     * Get R2 public URL for a key
     */
    public function getR2PublicUrl(string $key): string
    {
        // Use custom domain if configured, otherwise use R2 dev URL
        $publicUrl = $this->r2Config['public_url'] ?? '';
        
        if (empty($publicUrl)) {
            // Fallback to R2 dev domain
            $accountId = $this->r2Config['account_id'] ?? '';
            $bucket = $this->r2Config['bucket'] ?? '';
            $publicUrl = "https://{$bucket}.{$accountId}.r2.dev";
        }
        
        return rtrim($publicUrl, '/') . '/' . $key;
    }
    
    /**
     * Get Contabo public URL for a key
     */
    public function getContaboPublicUrl(string $key): string
    {
        $publicUrl = $this->contaboConfig['public_url'] ?? '';
        return rtrim($publicUrl, '/') . '/' . $key;
    }
    
    /**
     * Get storage status for admin dashboard
     */
    public function getStorageStatus(): array
    {
        $r2Usage = $this->getR2Usage();
        
        return [
            'r2' => [
                'enabled' => $this->r2Enabled,
                'usage' => $r2Usage,
                'status' => $r2Usage['is_full'] ? 'full' : ($r2Usage['is_warning'] ? 'warning' : 'ok'),
            ],
            'contabo' => [
                'enabled' => true,
                'status' => 'ok',
            ],
            'strategy' => [
                'thumb' => $this->r2Enabled && !$r2Usage['is_full'] ? 'r2' : 'contabo',
                'medium' => $this->r2Enabled && !$r2Usage['is_full'] ? 'r2' : 'contabo',
                'large' => 'contabo',
                'original' => 'contabo',
            ],
        ];
    }
    
    /**
     * Format bytes to human readable
     */
    private function formatBytes(int $bytes): string
    {
        $units = ['B', 'KB', 'MB', 'GB', 'TB'];
        $i = 0;
        while ($bytes >= 1024 && $i < count($units) - 1) {
            $bytes /= 1024;
            $i++;
        }
        return round($bytes, 2) . ' ' . $units[$i];
    }
    
    /**
     * Get free tier limit info
     */
    public static function getFreeTierInfo(): array
    {
        return [
            'storage_limit' => self::FREE_TIER_LIMIT,
            'storage_limit_human' => '9.5 GB (buffer from 10GB)',
            'warning_threshold' => self::WARNING_THRESHOLD,
            'warning_threshold_human' => '8 GB',
            'class_a_ops' => 1000000, // 1M write ops
            'class_b_ops' => 10000000, // 10M read ops
            'egress' => 'Unlimited',
        ];
    }
}
