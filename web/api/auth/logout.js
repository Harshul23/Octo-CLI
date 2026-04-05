import { neon } from '@neondatabase/serverless';

export default async function handler(req, res) {
  if (req.method !== 'POST') {
     return res.status(405).json({ error: 'Method not allowed' });
  }

  // Parse cookies
  let cookies = {};
  if (req.headers.cookie) {
    req.headers.cookie.split(';').forEach(cookie => {
      const parts = cookie.split('=');
      cookies[parts.shift().trim()] = decodeURI(parts.join('='));
    });
  }

  let token = cookies.session_token || req.cookies?.session_token;
  if (!token && req.headers.authorization?.startsWith('Bearer ')) {
      token = req.headers.authorization.split(' ')[1];
  }

  if (token && process.env.DATABASE_URL) {
      try {
          const sql = neon(process.env.DATABASE_URL);
          await sql`DELETE FROM sessions WHERE token = ${token}`;
      } catch (error) {
          console.error("Failed to delete session:", error);
          // Proceed to clear cookie anyway
      }
  }

  res.setHeader('Set-Cookie', [
      'session_token=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0',
      'oauth_state=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0'
  ]);
  return res.status(200).json({ message: 'Logged out successfully' });
}
