export default function handler(req, res) {
  // Use env variables or fallback to local headers for dynamic redirect
  const clientId = process.env.GITHUB_CLIENT_ID || process.env.VITE_GITHUB_CLIENT_ID;
  
  if (!clientId) {
    return res.status(500).json({ error: "Missing GitHub Client ID" });
  }

  // Determine host for callback url
  const proto = req.headers['x-forwarded-proto'] || 'http';
  const host = req.headers.host;
  let callbackUrl = process.env.GITHUB_CALLBACK_URL || process.env.VITE_GITHUB_CALLBACK_URL;
  
  // If no predefined callback, build it from host (helpful in Vercel previews)
  if (!callbackUrl && host) {
    callbackUrl = `${proto}://${host}/api/auth/github/callback`;
  }

  // Generate random state for CSRF protection
  const state = Math.random().toString(36).substring(2, 15);
  
  // Set state cookie
  res.setHeader('Set-Cookie', `oauth_state=${state}; Path=/; HttpOnly; SameSite=Lax; Max-Age=600`);
  
  const params = new URLSearchParams();
  params.append('client_id', clientId);
  params.append('redirect_uri', callbackUrl);
  params.append('scope', 'read:user user:email');
  params.append('state', state);
  
  res.redirect(`https://github.com/login/oauth/authorize?${params.toString()}`);
}
