// The STQ triangle list takes gsKit's GSPRIMSTQPOINT, a union over u128
// that cgo cannot express; the Go side passes the same bytes as void*.
#define _EE
#include <gsKit.h>
#include <gsToolkit.h>

void ps2go_prim_list_stq(GSGLOBAL *gs, GSTEXTURE *tex, int count, const void *vertices) {
	gsKit_prim_list_triangle_goraud_texture_stq_3d(gs, tex, count, (const GSPRIMSTQPOINT *)vertices);
}
