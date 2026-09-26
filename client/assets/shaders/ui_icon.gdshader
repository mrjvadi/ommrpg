shader_type canvas_item;
// Turns a white silhouette icon into a game-style icon: vertical colour
// gradient, dark outline and a soft drop shadow. The icon is drawn in the
// inner part of the rect so the outline never gets clipped.

uniform vec4 top_color : source_color = vec4(1.0, 0.9, 0.55, 1.0);
uniform vec4 bottom_color : source_color = vec4(0.85, 0.5, 0.15, 1.0);
uniform vec4 outline_color : source_color = vec4(0.07, 0.04, 0.03, 1.0);
uniform float outline_px = 3.0;
uniform float pad = 0.1;

float sample_a(sampler2D tex, vec2 uv) {
	vec2 q = (uv - vec2(pad)) / (1.0 - 2.0 * pad);
	if (q.x < 0.0 || q.y < 0.0 || q.x > 1.0 || q.y > 1.0) return 0.0;
	return texture(tex, q).a;
}

void fragment() {
	vec2 px = TEXTURE_PIXEL_SIZE * outline_px / (1.0 - 2.0 * pad);
	float a = sample_a(TEXTURE, UV);
	float o = 0.0;
	for (int i = 0; i < 12; i++) {
		float ang = float(i) * 0.5235988;
		o = max(o, sample_a(TEXTURE, UV + vec2(cos(ang), sin(ang)) * px));
	}
	float sh = sample_a(TEXTURE, UV - px * vec2(0.0, 1.6));
	vec3 fill = mix(top_color.rgb, bottom_color.rgb, clamp((UV.y - pad) / (1.0 - 2.0 * pad), 0.0, 1.0));
	// light rim on the top of the silhouette
	float lit = a * (1.0 - sample_a(TEXTURE, UV - px * vec2(0.0, 0.8)));
	fill = mix(fill, vec3(1.0), lit * 0.35);
	vec4 col = vec4(0.0, 0.0, 0.0, sh * 0.35);
	col = mix(col, vec4(outline_color.rgb, 1.0), o * outline_color.a);
	col = mix(col, vec4(fill, 1.0), a);
	COLOR = vec4(col.rgb, col.a * COLOR.a);
}
