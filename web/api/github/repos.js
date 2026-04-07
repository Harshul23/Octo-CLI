import { neon } from "@neondatabase/serverless";

export default async function handler(req, res) {
  if (req.method !== "GET") {
    return res.status(405).json({ error: "Method not allowed" });
  }

  // Parse cookies
  let cookies = {};
  if (req.headers.cookie) {
    req.headers.cookie.split(";").forEach((cookie) => {
      const parts = cookie.split("=");
      cookies[parts.shift().trim()] = decodeURI(parts.join("="));
    });
  }

  // Allow Bearer token
  let token = cookies.session_token || req.cookies?.session_token;
  if (!token && req.headers.authorization?.startsWith("Bearer ")) {
    token = req.headers.authorization.split(" ")[1];
  }

  if (!token) {
    return res.status(401).json({ error: "Not authenticated" });
  }

  try {
    let dbUrl = process.env.DATABASE_URL;
    if (!dbUrl) {
      return res.status(500).json({ error: "Missing DATABASE_URL" });
    }

    dbUrl = dbUrl.replace(/^["']|["']$/g, "");
    const sql = neon(dbUrl);

    // Validate session & get user's GitHub access token
    const users = await sql`
      SELECT u.access_token
      FROM users u
      JOIN sessions s ON u.id = s.user_id
      WHERE s.token = ${token} AND s.expires_at > now()
    `;

    if (users.length === 0) {
      return res
        .status(401)
        .json({ error: "Invalid session or user not found" });
    }

    const accessToken = users[0].access_token;
    if (!accessToken) {
      return res.status(400).json({ error: "GitHub not connected" });
    }

    // Fetch repositories from GitHub
    const ghRes = await fetch(
      "https://api.github.com/user/repos?sort=updated&per_page=50&type=all",
      {
        headers: {
          Authorization: `Bearer ${accessToken}`,
          Accept: "application/vnd.github.v3+json",
        },
      },
    );

    if (!ghRes.ok) {
      return res
        .status(ghRes.status)
        .json({ error: `GitHub API error (status ${ghRes.status})` });
    }

    const repos = await ghRes.json();
    return res.status(200).json({ repos });
  } catch (error) {
    console.error("GitHub fetch error:", error);
    return res.status(500).json({ error: "Internal server error" });
  }
}
