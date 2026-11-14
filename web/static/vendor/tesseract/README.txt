Tesseract OCR, bundled so the studio's "Image to text" never loads anything from another site.

tesseract.min.js, worker.min.js   tesseract.js 5.1.1 (Apache-2.0), unmodified, from the npm package dist/
tesseract-core-simd-lstm.wasm.js  tesseract.js-core 5.1.0 (Apache-2.0), unmodified (the WebAssembly build with SIMD, LSTM only)
lang/eng.traineddata.gz           English model, tessdata "4.0.0_best_int" from the npm package @tesseract.js-data/eng 1.0.0 (Apache-2.0)

Other reading languages are not bundled: the site owner puts <code>.traineddata.gz files in PIXELHOP_DATA/tessdata
(see scripts/ocr-language.sh). Browsers without WebAssembly SIMD cannot run this build.
