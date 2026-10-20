import { useEffect, useRef } from "react";
import type * as T from "three";
import { useReducedMotion } from "../lib/useReducedMotion";

/*
 * DRAFT. Thin black lines drifting behind the page, from the reference
 * posters: orbits, looping trajectories with arrow ticks (the "DF/2"
 * diagram), and with `cubes`, a few wireframe cubes around one red star.
 * Orthographic, line materials only: no lighting, no textures. Parallax on
 * the cursor; scrolling moves the camera down through the field.
 */
export function ThreeLines({ cubes = false }: { cubes?: boolean }) {
  const host = useRef<HTMLDivElement>(null);
  const reduced = useReducedMotion();

  useEffect(() => {
    const el = host.current;
    if (!el) return;
    let disposed = false;
    let cleanup = () => {};

    import("three").then((THREE) => {
      if (disposed) return;
      const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
      renderer.setPixelRatio(Math.min(2, window.devicePixelRatio));
      el.appendChild(renderer.domElement);
      const scene = new THREE.Scene();
      const camera = new THREE.OrthographicCamera(-1, 1, 1, -1, -100, 100);
      const ink = new THREE.LineBasicMaterial({ color: 0x000000, transparent: true, opacity: 0.22 });
      const faint = new THREE.LineBasicMaterial({ color: 0x000000, transparent: true, opacity: 0.1 });
      const red = new THREE.LineBasicMaterial({ color: 0xff0000 });
      const world = new THREE.Group();
      scene.add(world);

      const line = (pts: [number, number][], mat: T.LineBasicMaterial) =>
        new THREE.Line(new THREE.BufferGeometry().setFromPoints(pts.map(([x, y]) => new THREE.Vector3(x, y, 0))), mat);

      // Pseudo-random but fixed, so the field is the same on every visit.
      let seed = 7;
      const rnd = () => ((seed = (seed * 16807) % 2147483647) / 2147483647);

      const pageH = 60; // world units of scroll: the whole page
      for (let k = 0; k < 9; k++) {
        // orbits
        const cx = (rnd() - 0.5) * 18, cy = -rnd() * pageH, rx = 2 + rnd() * 7, ry = rx * (0.4 + rnd() * 0.6);
        const pts: [number, number][] = [];
        for (let a = 0; a <= Math.PI * 2 + 0.01; a += 0.04) pts.push([cx + Math.cos(a) * rx, cy + Math.sin(a) * ry]);
        world.add(line(pts, k % 3 ? faint : ink));
      }
      for (let k = 0; k < 6; k++) {
        // looping trajectories: a trochoid running across, as in the "DF/2" plate
        const y0 = -rnd() * pageH, r = 0.6 + rnd() * 0.8, d = r * (1.3 + rnd()), step = 2.2 + rnd() * 1.5;
        const pts: [number, number][] = [];
        for (let t = 0; t < 24; t += 0.03) pts.push([-12 + (step / (2 * Math.PI)) * t * 1.6 - d * Math.sin(t), y0 + r - d * Math.cos(t) * 0.7]);
        world.add(line(pts, k === 0 ? red : ink));
        // arrow ticks along its axis
        for (let x = -11; x < 12; x += 4) world.add(line([[x - 0.25, y0 + 0.25], [x, y0], [x - 0.25, y0 - 0.25]], ink));
      }
      for (let k = 0; k < 14; k++) {
        const y = -rnd() * pageH;
        world.add(line([[-14, y], [14, y + (rnd() - 0.5) * 3]], faint));
      }

      const spinners: T.Object3D[] = [];
      if (cubes) {
        const edges = new THREE.EdgesGeometry(new THREE.BoxGeometry(1, 1, 1));
        for (let k = 0; k < 7; k++) {
          const c = new THREE.LineSegments(edges, k === 0 ? red : ink);
          const s = 0.8 + rnd() * 1.6;
          c.scale.set(s, s, s);
          c.position.set((rnd() - 0.5) * 18, -rnd() * pageH, 0);
          c.rotation.set(rnd() * 3, rnd() * 3, 0);
          world.add(c);
          spinners.push(c);
        }
        // the red four-point star
        const star: [number, number][] = [];
        for (let a = 0; a <= Math.PI * 2 + 0.01; a += Math.PI / 4) {
          const rr = Math.round(a / (Math.PI / 4)) % 2 ? 0.18 : 1;
          star.push([Math.cos(a) * rr, Math.sin(a) * rr]);
        }
        const st = line(star, red);
        st.position.set(5, -14, 0);
        world.add(st);
      }

      const pointer = { x: 0, y: 0 };
      const onMove = (e: PointerEvent) => {
        pointer.x = e.clientX / window.innerWidth - 0.5;
        pointer.y = e.clientY / window.innerHeight - 0.5;
      };
      window.addEventListener("pointermove", onMove);

      const resize = () => {
        const w = window.innerWidth, h = window.innerHeight;
        renderer.setSize(w, h);
        const half = 13; // world units across half the width
        camera.left = -half; camera.right = half;
        camera.top = (half * h) / w; camera.bottom = (-half * h) / w;
        camera.updateProjectionMatrix();
      };
      resize();
      window.addEventListener("resize", resize);

      let frame = 0;
      let last = 0;
      const tick = (now: number) => {
        frame = requestAnimationFrame(tick);
        if (document.hidden || now - last < 33) return; // about 30fps is plenty for drift
        last = now;
        const scrollMax = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
        const progress = window.scrollY / scrollMax;
        camera.position.y = -progress * (pageH - 8) - 4 + pointer.y * -0.6;
        camera.position.x = pointer.x * 0.8;
        if (!reduced) {
          world.rotation.z = Math.sin(now / 9000) * 0.015;
          for (const c of spinners) { c.rotation.x += 0.003; c.rotation.y += 0.004; }
        }
        renderer.render(scene, camera);
      };
      frame = requestAnimationFrame(tick);

      cleanup = () => {
        cancelAnimationFrame(frame);
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("resize", resize);
        renderer.dispose();
        renderer.domElement.remove();
      };
    });
    return () => {
      disposed = true;
      cleanup();
    };
  }, [cubes, reduced]);

  return <div ref={host} aria-hidden="true" style={{ position: "fixed", inset: 0, pointerEvents: "none", zIndex: 0 }} />;
}
