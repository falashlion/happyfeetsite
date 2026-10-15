// Swatch colours for the colour names the console offers. SKUs store a name
// and usually no hex, and a grey dot for every colourway tells a shopper
// nothing — so the name is resolved here. Unknown names stay neutral.
export const COLOR_HEX: Record<string, string> = {
  black: "#15161B",
  white: "#F7F7F5",
  brown: "#6B4423",
  tan: "#B5835A",
  navy: "#1B2A4A",
  grey: "#8C8E96",
  gray: "#8C8E96",
  beige: "#E3D5B8",
  cream: "#F5EFE0",
  red: "#B3261E",
  green: "#2F5D3A",
  blue: "#2F5DA8",
  gold: "#C8A24A",
  silver: "#C0C2C7",
  pink: "#E8A3B5",
  orange: "#D9732B",
  yellow: "#E8C547",
  purple: "#6B4C9A",
  burgundy: "#6D1F2F",
  khaki: "#A89A6A",
};

export const COLOR_NAMES = [
  "Black", "White", "Brown", "Tan", "Navy", "Grey", "Beige", "Cream",
  "Red", "Green", "Blue", "Gold", "Silver", "Pink", "Orange", "Burgundy",
];

export function colorHex(name: string, stored?: string | null): string {
  return stored || COLOR_HEX[name.trim().toLowerCase()] || "#888888";
}
