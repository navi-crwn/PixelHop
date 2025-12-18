<?php
/**
 * S3 Object Storage Configuration Example
 * Copy this to s3.php and update with your credentials
 * 
 * Hybrid Storage Strategy:
 * - Cloudflare R2: Thumbnails & Medium (free tier, zero egress)
 * - Contabo/Other S3: Original & Large (unlimited storage)
 */

return [
    // Primary storage: Your main S3-compatible storage
    's3' => [
        'endpoint' => 'https://your-s3-endpoint.com',
        'region' => 'default',
        'bucket' => 'your-bucket-name',
        'access_key' => 'your-access-key',
        'secret_key' => 'your-secret-key',
        'use_path_style' => true,
        'public_url' => 'https://your-s3-endpoint.com/your-bucket',
    ],

    // Secondary storage: Cloudflare R2 (optional, free tier)
    // Enable this to use R2 for thumbnails (saves bandwidth)
    'r2' => [
        'enabled' => false, // Set to true after configuring
        'account_id' => 'your-cloudflare-account-id',
        'endpoint' => 'https://<ACCOUNT_ID>.r2.cloudflarestorage.com',
        'region' => 'auto',
        'bucket' => 'your-r2-bucket',
        'access_key' => 'r2-access-key',
        'secret_key' => 'r2-secret-key',
        'public_url' => 'https://your-r2-custom-domain.com',
        
        // Safety limits (stay within free tier)
        'max_storage_bytes' => 9.5 * 1024 * 1024 * 1024, // 9.5GB
        'warning_threshold' => 8 * 1024 * 1024 * 1024,   // 8GB
    ],

    'upload' => [
        'max_size' => 10 * 1024 * 1024,
        'allowed_types' => ['image/jpeg', 'image/png', 'image/gif', 'image/webp'],
        'allowed_extensions' => ['jpg', 'jpeg', 'png', 'gif', 'webp'],
    ],

    'image' => [
        'quality' => 85,
        'sizes' => [
            'thumb' => ['width' => 150, 'height' => 150],
            'medium' => ['width' => 600, 'height' => 600],
            'large' => ['width' => 1200, 'height' => 1200],
        ],
    ],

    'site' => [
        'name' => 'PixelHop',
        'domain' => 'your-domain.com',
        'url' => 'https://your-domain.com',
    ],
];
