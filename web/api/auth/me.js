import { neon } from '@neondatabase/serverless';

export default async function handler(req, res) {
  // Parse cookies
  let cookies = {};
  if (req.headers.cookie) {
    req.headers.cookie.split(';').forEach(cookie => {
      const parts = cookie.split('=');
      cookies[parts.shift().trim()] = decodeURI(parts.join('='));
    });
  }

  // Allow Bearer token as well
  let token = cookies.session_token || req.cookies?.session_token;
  if (!token && req.headers.authorization?.startsWith('Bearer ')) {
      token = req.headers.authorization.split(' ')[1];
  }

  if (!token) {
    return res.status(401).json({ error: 'Not authenticated' });
  }

  try {
    if (!process.env.DATABASE_URL) {
        return res.status(500).json({ error: 'Missing DATABASE_URL' });
    }
    const sql = neon(process.env.DATABASE_URL);
    
    // Validate session
    const sessions = await sql`
        SELECT user_id FROM sessions 
        WHERE token = ${token} AND expires_at > now()
    `;
    
    if (sessions.length === 0) {
        return res.status(401).json({ error: 'Invalid or expired session' });
    }
    
    const userId = sessions[0].user_id;
    
    // Get user info (excluding access_token for security)
    const users = await sql`
        SELECT id, github_id as "githubId", email, name, avatar_url, github_username, created_at, updated_at
        FROM users 
        WHERE id = ${userId}
    `;
    
    if (users.length === 0) {
         return res.status(401).json({ error: 'User not found' });
    }
    
    return res.status(200).json(users[0]);
  } catch (error) {
    console.error('Auth me error:', error);
    return res.status(500).json({ error: 'Internal server error' });
  }
}
