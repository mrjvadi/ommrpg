// Weapon effect drawn on top of an LPC character, using the sprite-service
// weapon mask (white where the visible pixel belongs to the weapon).
// - outer glow: dilated mask with distance falloff, flickering
// - inner shimmer: bright bands sweeping along the blade
// - element styles: fire flickers upward, frost pulses slowly, lightning
//   crackles, poison bubbles, holy radiates, shadow wavers
shader_type canvas_item;
render_mode blend_add;

uniform vec4 glow_color : source_color = vec4(1.0, 0.6, 0.2, 1.0);
uniform float intensity = 1.0;   // grows with the enhancement tier
uniform float radius = 2.0;      // glow radius in pixels (tier based)
uniform int style = 0;           // 0 none/rarity 1 fire 2 frost 3 lightning 4 poison 5 holy 6 shadow

float hash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }

void fragment() {
	vec2 px = TEXTURE_PIXEL_SIZE;
	float a = texture(TEXTURE, UV).a;
	float glow = 0.0;
	for (int x = -3; x <= 3; x++) {
		for (int y = -3; y <= 3; y++) {
			float d = length(vec2(float(x), float(y)));
			if (d > radius + 0.5) continue;
			float m = texture(TEXTURE, UV + vec2(float(x), float(y)) * px).a;
			glow = max(glow, m * (1.0 - d / (radius + 1.0)));
		}
	}
	float t = TIME;
	vec2 cell = floor(UV / px);
	float flicker = 0.8 + 0.2 * sin(t * 7.0 + cell.y * 0.7);
	if (style == 1) flicker = 0.65 + 0.35 * hash(vec2(cell.x, floor(cell.y + t * 12.0)));      // fire licks upward
	if (style == 2) flicker = 0.75 + 0.25 * sin(t * 2.0);                                      // frost pulse
	if (style == 3) flicker = step(0.35, hash(vec2(floor(t * 20.0), cell.x + cell.y))) * 1.2;  // lightning crackle
	if (style == 4) flicker = 0.7 + 0.3 * sin(t * 5.0 + hash(cell) * 6.28);                    // poison bubbles
	if (style == 5) flicker = 0.9 + 0.1 * sin(t * 3.0);                                        // holy steady radiance
	if (style == 6) flicker = 0.6 + 0.4 * sin(t * 4.0 + cell.x * 0.5);                         // shadow waver
	float outer = (1.0 - a) * glow * flicker;
	float band = a * max(0.0, sin((cell.x + cell.y) * 0.35 - t * 6.0)) * 0.8;
	float alpha = clamp((outer * 1.1 + band) * intensity, 0.0, 1.0);
	COLOR = vec4(glow_color.rgb * alpha, alpha);
}
