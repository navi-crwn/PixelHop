<?php
/**
 * PixelHop - Resend Verification Email
 * POST /auth/resend-verification.php
 */

header('Content-Type: application/json');
session_start();

require_once __DIR__ . '/../includes/Database.php';
require_once __DIR__ . '/../includes/Mailer.php';

// Rate limiting - 1 request per minute
$rateLimitKey = 'resend_verification_' . ($_SERVER['REMOTE_ADDR'] ?? 'unknown');
$lastRequest = $_SESSION[$rateLimitKey] ?? 0;

if (time() - $lastRequest < 60) {
    $remaining = 60 - (time() - $lastRequest);
    echo json_encode(['success' => false, 'message' => "Please wait {$remaining} seconds"]);
    exit;
}

// Get email from POST or session
$input = json_decode(file_get_contents('php://input'), true) ?? [];
$email = trim($input['email'] ?? $_SESSION['pending_verification_email'] ?? '');

if (empty($email)) {
    echo json_encode(['success' => false, 'message' => 'Email address required']);
    exit;
}

try {
    $db = Database::getInstance();
    
    // Find unverified user
    $stmt = $db->prepare("
        SELECT id, email, email_verified 
        FROM users 
        WHERE email = ? 
        LIMIT 1
    ");
    $stmt->execute([$email]);
    $user = $stmt->fetch(PDO::FETCH_ASSOC);
    
    if (!$user) {
        // Don't reveal if email exists or not
        echo json_encode(['success' => true, 'message' => 'If the email exists, a verification link has been sent']);
        exit;
    }
    
    if ($user['email_verified']) {
        echo json_encode(['success' => false, 'message' => 'Email already verified. You can login.']);
        exit;
    }
    
    // Generate new verification token
    $verificationToken = bin2hex(random_bytes(32));
    $verificationExpires = date('Y-m-d H:i:s', strtotime('+24 hours'));
    
    $updateStmt = $db->prepare("
        UPDATE users 
        SET email_verification_token = ?, 
            email_verification_expires = ? 
        WHERE id = ?
    ");
    $updateStmt->execute([$verificationToken, $verificationExpires, $user['id']]);
    
    // Send verification email
    $mailer = new Mailer();
    $emailSent = $mailer->sendVerificationEmail($email, $verificationToken);
    
    // Update rate limit
    $_SESSION[$rateLimitKey] = time();
    
    if ($emailSent) {
        echo json_encode(['success' => true, 'message' => 'Verification email sent']);
    } else {
        echo json_encode(['success' => false, 'message' => 'Failed to send email. Please try again.']);
    }
    
} catch (Exception $e) {
    error_log('Resend verification error: ' . $e->getMessage());
    echo json_encode(['success' => false, 'message' => 'An error occurred']);
}
