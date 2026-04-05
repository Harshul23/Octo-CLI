import { neon } from '@neondatabase/serverless';

export default async function handler(req, res) {
  const { code, state } = req.query;
  
  // Parse cookies from headers since req.cookies might not be fully populated depending on config
  let cookies = {};
  if (req.headers.cookie) {
    req.headers.cookie.split(';').forEach(cookie => {
      const parts = cookie.split('=');
      cookies[parts.shift().trim()] = decodeURI(parts.join('='));
    });
  }
  
  const stateCookie = req.cookies?.oauth_state || cookies.oauth_state;

  if (!stateCookie) {
    return res.status(400).send('Missing state cookie');
  }

  if (state !== stateCookie) {
    return res.status(400).send('Invalid state parameter');
  }

  const clientId = process.env.GITHUB_CLIENT_ID || process.env.VITE_GITHUB_CLIENT_ID;
  const clientSecret = process.env.GITHUB_CLIENT_SECRET;

  if (!clientId || !clientSecret) {
     return res.status(500).send('Missing GitHub credentials in environment variables');
  }

  try {
    // Exchange code for token
    const tokenRes = await fetch('https://github.com/login/oauth/access_token', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
      },
      body: JSON.stringify({
        client_id: clientId,
        client_secret: clientSecret,
        code
      })
    });
    
    const tokenData = await tokenRes.json();
    if (tokenData.error) {
      return res.status(400).send('OAuth error: ' + tokenData.error_description);
    }
    
    const accessToken = tokenData.access_token;
    
    // Get GitHub user info
    const userRes = await fetch('https://api.github.com/user', {
      headers: {
        'Authorization': `Bearer ${accessToken}`,
        'Accept': 'application/vnd.github+json'
      }
    });
    
    if (!userRes.ok) {
        return res.status(500).send('Failed to get user info from GitHub');
    }
    
    const githubUser = await userRes.json();
    
    // Get user email
    let email = githubUser.email;
    if (!email) {
      const emailRes = await fetch('https://api.github.com/user/emails', {
        headers: {
          'Authorization': `Bearer ${accessToken}`,
          'Accept': 'application/vnd.github+json'
        }
      });
      if (emailRes.ok) {
          const emails = await emailRes.json();
          const primary = emails.find(e => e.primary && e.verified) || emails.find(e => e.verified) || (emails.length > 0 ? emails[0] : null);
          email = primary ? primary.email : '';
      } else {
          email = '';
      }
    }

    // Connect to Neon Database
    if (!process.env.DATABASE_URL) {
        return res.status(500).send('Missing DATABASE_URL');
    }
    
    const sql = neon(process.env.DATABASE_URL);
    
    // Create or update user
    const users = await sql`
      INSERT INTO users (github_id, email, name, avatar_url, github_username, access_token)
      VALUES (${githubUser.id.toString()}, ${email}, ${githubUser.name || githubUser.login}, ${githubUser.avatar_url}, ${githubUser.login}, ${accessToken})
      ON CONFLICT (github_id) DO UPDATE SET
        email = EXCLUDED.email,
        name = EXCLUDED.name,
        avatar_url = EXCLUDED.avatar_url,
        github_username = EXCLUDED.github_username,
        access_token = EXCLUDED.access_token,
        updated_at = now()
      RETURNING id
    `;
    const userId = users[0].id;
    
    // Create session
    const sessionToken = crypto.randomUUID() + crypto.randomUUID(); // secure enough for this
    const expiresAt = new Date(Date.now() + 30 * 24 * 3600 * 1000).toISOString();
    
    await sql`
      INSERT INTO sessions (user_id, token, expires_at)
      VALUES (${userId}, ${sessionToken}, ${expiresAt})
    `;
    
    // Set session cookie and clear oauth_state
    const cookieHeader = [
      'oauth_state=; Path=/; HttpOnly; SameSite=Lax; Max-Age=0',
      `session_token=${sessionToken}; Path=/; HttpOnly; SameSite=Lax; Max-Age=2592000` // 30 days
    ];
    res.setHeader('Set-Cookie', cookieHeader);
    
    // Redirect to frontend dashboard
    const proto = req.headers['x-forwarded-proto'] || 'http';
    const host = req.headers.host;
    const redirectUrl = `${proto}://${host}/dashboard?token=${sessionToken}`;
    
    res.redirect(redirectUrl);

  } catch (error) {
    console.error('Callback error:', error);
    res.status(500).send('Internal Server Error: ' + error.message);
  }
}
