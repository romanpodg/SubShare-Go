"use client";

import { useEffect, useRef, useState } from "react";
import styles from "./NumericGlobe.module.css";

export interface NumericGlobePoint {
  x: number;
  y: number;
  z: number;
  digit: string;
  accent: boolean;
}

export interface ProjectedNumericGlobePoint extends NumericGlobePoint {
  projectedX: number;
  projectedY: number;
  depth: number;
  scale: number;
  opacity: number;
}

const DEFAULT_POINT_COUNT = 260;
const GOLDEN_ANGLE = Math.PI * (3 - Math.sqrt(5));
const STATIC_ANGLE = 0.38;
const ROTATION_SPEED = 0.000085;

export function createNumericGlobePoints(count = DEFAULT_POINT_COUNT): NumericGlobePoint[] {
  if (count <= 0) return [];
  if (count === 1) {
    return [{ x: 0, y: 0, z: 1, digit: "7", accent: true }];
  }

  return Array.from({ length: count }, (_, index) => {
    const y = 1 - (index / (count - 1)) * 2;
    const latitudeRadius = Math.sqrt(Math.max(0, 1 - y * y));
    const longitude = index * GOLDEN_ANGLE;

    return {
      x: Math.cos(longitude) * latitudeRadius,
      y,
      z: Math.sin(longitude) * latitudeRadius,
      digit: String((index * 7 + 3) % 10),
      accent: index % 47 === 0,
    };
  });
}

export function projectNumericGlobePoint(
  point: NumericGlobePoint,
  angle: number
): ProjectedNumericGlobePoint {
  const cosine = Math.cos(angle);
  const sine = Math.sin(angle);
  const rotatedX = point.x * cosine + point.z * sine;
  const rotatedZ = -point.x * sine + point.z * cosine;
  const perspective = 0.86 + (rotatedZ + 1) * 0.07;
  const normalizedDepth = (rotatedZ + 1) / 2;

  return {
    ...point,
    projectedX: rotatedX * perspective,
    projectedY: point.y * perspective,
    depth: rotatedZ,
    scale: 0.68 + normalizedDepth * 0.58,
    opacity: 0.12 + normalizedDepth * 0.8,
  };
}

function combineClassNames(...classNames: Array<string | undefined>) {
  return classNames.filter(Boolean).join(" ");
}

export function NumericGlobe({ className }: { className?: string }) {
  const rootRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const pointsRef = useRef(createNumericGlobePoints());
  const [motionMode, setMotionMode] = useState<"animated" | "static">("animated");

  useEffect(() => {
    const root = rootRef.current;
    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!root || !canvas || !context) return;

    const motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
    let reducedMotion = motionQuery.matches;
    let animationFrame = 0;
    let animationStartedAt = 0;
    let width = 0;
    let height = 0;

    const syncCanvasSize = () => {
      const bounds = root.getBoundingClientRect();
      const nextWidth = Math.max(1, Math.round(bounds.width));
      const nextHeight = Math.max(1, Math.round(bounds.height));
      const pixelRatio = Math.min(window.devicePixelRatio || 1, 2);

      width = nextWidth;
      height = nextHeight;
      canvas.width = Math.round(nextWidth * pixelRatio);
      canvas.height = Math.round(nextHeight * pixelRatio);
      context.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
    };

    const draw = (angle: number) => {
      if (width === 0 || height === 0) syncCanvasSize();

      context.clearRect(0, 0, width, height);
      context.textAlign = "center";
      context.textBaseline = "middle";

      const centerX = width / 2;
      const centerY = height / 2;
      const radius = Math.min(width, height) * 0.37;
      const projectedPoints = pointsRef.current
        .map((point) => projectNumericGlobePoint(point, angle))
        .sort((left, right) => left.depth - right.depth);

      for (const point of projectedPoints) {
        const fontSize = Math.max(6, radius * 0.05 * point.scale);
        context.globalAlpha = point.opacity;
        context.fillStyle = point.accent ? "#b7ff2a" : "#f2f3ef";
        context.font = `${point.depth > 0.58 ? 600 : 450} ${fontSize}px "JetBrains Mono Variable", "Cascadia Code", monospace`;
        context.fillText(
          point.digit,
          centerX + point.projectedX * radius,
          centerY + point.projectedY * radius
        );
      }

      context.globalAlpha = 1;
    };

    const stopAnimation = () => {
      if (animationFrame) {
        window.cancelAnimationFrame(animationFrame);
        animationFrame = 0;
      }
    };

    const animate = (timestamp: number) => {
      if (!animationStartedAt) animationStartedAt = timestamp;
      draw(STATIC_ANGLE + (timestamp - animationStartedAt) * ROTATION_SPEED);
      animationFrame = window.requestAnimationFrame(animate);
    };

    const startAnimation = () => {
      stopAnimation();
      setMotionMode(reducedMotion ? "static" : "animated");

      if (reducedMotion || document.hidden) {
        draw(STATIC_ANGLE);
        return;
      }

      animationStartedAt = 0;
      animationFrame = window.requestAnimationFrame(animate);
    };

    const handleMotionPreference = (event: MediaQueryListEvent) => {
      reducedMotion = event.matches;
      startAnimation();
    };

    const handleVisibilityChange = () => {
      if (document.hidden) {
        stopAnimation();
      } else {
        startAnimation();
      }
    };

    const resizeObserver =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(() => {
            syncCanvasSize();
            if (reducedMotion || document.hidden) draw(STATIC_ANGLE);
          });

    syncCanvasSize();
    resizeObserver?.observe(root);
    window.addEventListener("resize", syncCanvasSize);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    motionQuery.addEventListener("change", handleMotionPreference);
    startAnimation();

    return () => {
      stopAnimation();
      resizeObserver?.disconnect();
      window.removeEventListener("resize", syncCanvasSize);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      motionQuery.removeEventListener("change", handleMotionPreference);
    };
  }, []);

  return (
    <div
      ref={rootRef}
      className={combineClassNames(styles.globe, className)}
      role="img"
      aria-label="Цифровой глобус сети SubShare"
      data-testid="numeric-globe"
      data-motion={motionMode}
    >
      <canvas ref={canvasRef} className={styles.canvas} aria-hidden="true" />
    </div>
  );
}
