/**
 * Octo Logo component — renders the actual SVG logo from assets.
 * Falls back to emoji 🐙 if image fails to load.
 */
export default function OctoLogo({ size = 32, className = "" }) {
  return (
    <img
      src="/octo-icon.svg"
      alt="Octo"
      width={size}
      height={size}
      className={className}
      onError={(e) => {
        e.target.style.display = "none";
        e.target.parentElement.insertAdjacentHTML(
          "afterbegin",
          `<span style="font-size:${size * 0.6}px;line-height:${size}px">🐙</span>`,
        );
      }}
    />
  );
}
