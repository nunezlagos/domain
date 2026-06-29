(function () {
  'use strict';

  var s = getComputedStyle(document.documentElement);
  function v(n) { return s.getPropertyValue(n).trim(); }

  window.DJ = {
    /* colores de gráficas — sólidos */
    c1:   v('--dj-chart-c1'),
    c2:   v('--dj-chart-c2'),
    c3:   v('--dj-chart-c3'),
    c4:   v('--dj-chart-c4'),
    c5:   v('--dj-chart-c5'),
    /* alfas de c1 */
    c1a10: v('--dj-chart-c1-a10'),
    c1a40: v('--dj-chart-c1-a40'),
    c1a70: v('--dj-chart-c1-a70'),
    /* alfas de c2 */
    c2a60: v('--dj-chart-c2-a60'),
    c2a70: v('--dj-chart-c2-a70'),
    /* alfas de c3 */
    c3a60: v('--dj-chart-c3-a60'),
    c3a70: v('--dj-chart-c3-a70'),
    /* alfas de c4 */
    c4a10: v('--dj-chart-c4-a10'),
    c4a70: v('--dj-chart-c4-a70'),
    /* alfas de c5 */
    c5a70: v('--dj-chart-c5-a70'),
    /* texto para ejes y leyendas */
    textMuted:  v('--dj-text-muted'),
    textDim:    v('--dj-text-dim'),
    textFaint:  v('--dj-text-faint'),
    textMid:    v('--dj-text-mid'),
    textStrong: v('--dj-text-strong'),
    /* grilla de ejes */
    gridLine: v('--dj-chart-grid'),
    /* success */
    success:     v('--dj-chart-success'),
    successA25:  v('--dj-chart-success-a25'),
    successGrid: v('--dj-chart-success-a08'),
    /* segmentos del nav (mapeo c1–c8) */
    segments: [
      v('--dj-chart-c1'), /* agents   */
      v('--dj-chart-c2'), /* skills   */
      v('--dj-chart-c3'), /* flows    */
      v('--dj-chart-c4'), /* prompts  */
      v('--dj-c5'),       /* projects */
      v('--dj-c6'),       /* users    */
      v('--dj-c7'),       /* policies */
      v('--dj-c8'),       /* crons    */
    ],
  };
})();
