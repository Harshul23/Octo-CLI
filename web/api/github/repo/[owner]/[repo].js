import { neon } from "@neondatabase/serverless";

export default async function handler(req, res) {
  if (req.method !== "GET") {
    return res.status(405).json({ error: "Method not allowed" });
  }

  // Ex: /api/github/repo/[owner]/[repo]
  // Extract owner and repo from query
  const { owner, repo } = req.query;

  if (!owner || !repo) {
    return res.status(400).json({ error: "Missing owner or repo parameters" });
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

  let accessToken = null;

  try {
    if (token) {
      let dbUrl = process.env.DATABASE_URL;
      if (dbUrl) {
        dbUrl = dbUrl.replace(/^["']|["']$/g, "");
        const sql = neon(dbUrl);

        // Validate session & get user's GitHub access token
        const users = await sql`
          SELECT u.access_token
          FROM users u
          JOIN sessions s ON u.id = s.user_id
          WHERE s.token = ${token} AND s.expires_at > now()
        `;

        if (users.length > 0) {
          accessToken = users[0].access_token;
        }
      }
    }

    // Fetch repository info from GitHub
    const url = `https://api.github.com/repos/${owner}/${repo}`;
    const headers = {
      Accept: "application/vnd.github.v3+json",
    };

    if (accessToken) {
      headers["Authorization"] = `Bearer ${accessToken}`;
    }

    const ghRes = await fetch(url, { headers });

    if (!ghRes.ok) {
      return res
        .status(ghRes.status)
        .json({ error: `GitHub API error (status ${ghRes.status})` });
    }

    const repoInfo = await ghRes.json();
    return res.status(200).json(repoInfo);
  } catch (error) {
    console.error("GitHub fetch error:", error);
    return res.status(500).json({ error: "Internal server error" });
  }
}
