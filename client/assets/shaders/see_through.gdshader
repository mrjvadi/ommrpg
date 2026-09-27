shader_type canvas_item;
// "See-through circle": props drawn over the player become transparent
// inside a soft circle around the player, so trees and rocks fade only
// where they would hide the hero (the rest of the canopy stays solid).
// `center` is the player's chest in canvas (global 2D) coordinates.

uniform vec2 center = vec2(0.0);
uniform float radius = 64.0;
uniform float softness = 30.0;
uniform float min_alpha = 0.3;

varying vec2 world_pos;

void vertex() {
	world_pos = (MODEL_MATRIX * vec4(VERTEX, 0.0, 1.0)).xy;
}

void fragment() {
	vec4 c = texture(TEXTURE, UV) * COLOR;
	float d = distance(world_pos, center);
	float k = smoothstep(radius - softness, radius, d);
	// inside the circle: see-through and a little darker, like glass
	c.rgb *= mix(0.85, 1.0, k);
	c.a *= mix(min_alpha, 1.0, k);
	COLOR = c;
}
