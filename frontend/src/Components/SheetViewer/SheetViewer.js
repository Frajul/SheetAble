/*
	This file needs to be rewritten soon due to being not properly readable anymore
*/

import React, { useEffect, useState, Fragment, useRef } from "react";
import { useParams } from "react-router-dom";

import { Document, pdfjs, Page } from "react-pdf";

import SideBar from "../Sidebar/SideBar";
import "./SheetViewer.css";

import axios from "axios";

/* Utils */
import {
  displayTimeAsString,
  findSheetByPages,
  findComposerByPages,
  findSheetBySheets,
  findComposerByComposers,
  getCompImgUrl,
} from "../../Utils/utils";

/* Redux stuff */
import { connect } from "react-redux";
import { store } from "../../Redux/store";
import { logoutUser } from "../../Redux/Actions/userActions";
import {
  getComposerPage,
  getSheetPage,
  setSheetPage,
  setComposerPage,
} from "../../Redux/Actions/dataActions";
import { useHistory } from "react-router-dom";

import Modal from "../Sidebar/Modal/Modal";
import ModalContent from "./Components/ModalContent";
import InformationCard from "./Components/InformationCard";

/* Activate global worker for displaying the pdf properly */
pdfjs.GlobalWorkerOptions.workerSrc = `//cdnjs.cloudflare.com/ajax/libs/pdf.js/${pdfjs.version}/pdf.worker.js`;

function SheetViewer({
  sheetPages,
  composerPages,
  sheets,
  composers,
  sheetPage,
  getSheetPage,
  totalSheetPages,
  setSheetPage,
  getComposerPage,
  composerPage,
  totalComposerPages,
  setComposerPage,
}) {
  /* PDF Page width rendering */

  const windowHeight = 840;

  const [isDesktop, setDesktop] = useState(window.innerHeight > windowHeight);
  const documentRef = useRef(null);
  const [documentSize, setDocumentSize] = useState({ width: 0, height: 0 });
  const [pdfDimensionsLeft, setPdfDimensionsLeft] = useState({ width: 0, height: 0 });
  const [pdfDimensionsRight, setPdfDimensionsRight] = useState({ width: 0, height: 0 });
  const [isFullscreen, setIsFullscreen] = useState(false);

  const updateMedia = () => {
    const nextDesktop = window.innerHeight > windowHeight;
    setDesktop(nextDesktop);
  };
  const updateDocumentSize = () => {
    if (documentRef.current) {
      const { clientWidth, clientHeight } = documentRef.current;
      setDocumentSize({ width: clientWidth, height: clientHeight });
    }
  };

  useEffect(() => {
    updateDocumentSize(); // Update size on mount

    // Change Page Title
    document.title = `SheetAble - ${
      sheet.sheet_name === undefined ? "Sheet" : sheet.sheet_name
    }`;

    window.addEventListener("resize", updateDocumentSize);
    window.addEventListener("resize", updateMedia);

    return () => {
      window.removeEventListener("resize", updateDocumentSize);
      window.removeEventListener("resize", updateMedia);
    };
  });

  let { safeSheetName, safeComposerName } = useParams();

  const getSheetDataReq = async (_callback) => {
    if (
      sheetPage === undefined ||
      sheetPages < 0 ||
      sheetPages > totalSheetPages
    ) {
      setSheetPage(1);
    }

    const data = {
      page: sheetPage,
      sortBy: "updated_at desc",
    };

    if (sheetPages === undefined || sheetPages[sheetPage] === undefined) {
      await getSheetPage(data, () => window.location.reload());
    }
  };

  const getComposerDataReq = async (_callback) => {
    if (
      composerPage === undefined ||
      composerPages < 0 ||
      composerPages > totalComposerPages
    ) {
      setComposerPage(1);
    }

    const data = {
      page: composerPage,
      sortBy: "updated_at desc",
    };

    if (
      composerPages === undefined ||
      composerPages[composerPage] === undefined
    ) {
      await getComposerPage(data, () => window.location.reload());
    }
  };

  const [pdf, setpdf] = useState(undefined);
  const [twoPageMode, setTwoPageMode] = useState(true);

  const bySheetPages = findSheetByPages(safeSheetName, sheetPages);
  const bySheets = findSheetBySheets(safeSheetName, sheets);

  const [sheet] = useState(
    bySheetPages === undefined
      ? bySheets === undefined
        ? getSheetDataReq()
        : bySheets
      : bySheetPages
  );

  const byComposerPages = findComposerByPages(safeComposerName, composerPages);
  const byComposers = findComposerByComposers(safeComposerName, composers);

  const [composer] = useState(
    byComposerPages === undefined
      ? byComposers === undefined
        ? getComposerDataReq()
        : byComposers
      : byComposerPages
  );

  const pdfRequest = () => {
    axios
      .get(`/sheet/pdf/${safeComposerName}/${safeSheetName}`, {
        responseType: "arraybuffer",
      })
      .then((res) => {
        setpdf(res);
      })
      .catch((err) => {
        if (err.request.status === 401) {
          store.dispatch(logoutUser());
          window.location.href = "/login";
        }
        if (err.request.status === 404) {
          window.location.href = "/";
        }
      });
  };

 useEffect(() => {
    if (safeComposerName && safeSheetName && !pdf) {
      pdfRequest();
    }
  }, [safeComposerName, safeSheetName, pdf]);

  const [numPages, setNumPages] = useState(null);
  const [pageNumber, setPageNumber] = useState(1);

  function onDocumentLoadSuccess(pdf) {
    setNumPages(pdf.numPages);
    setPageNumber(1);
  }

  function onPdfPageLoadSuccessLeft(page) {
      const { width, height } = page.getViewport({ scale: 1 });
      setPdfDimensionsLeft({ width, height });
  }

  function onPdfPageLoadSuccessRight(page) {
      const { width, height } = page.getViewport({ scale: 1 });
      setPdfDimensionsRight({ width, height });
  }

  function changePageIfPossible(offset) {
    if (pageNumber + offset <= 0) {
      offset = 1 - pageNumber;
    }
    else if (offset == 2 && pageNumber == numPages -1) {
      return; // Do not allow to page advance when right page is last page
    }
    else if (pageNumber + offset > numPages) {
      offset = numPages - pageNumber;
    }
    setPageNumber((prevPageNumber) => prevPageNumber + offset);
  }

  let history = useHistory();

  const [pdfDownloadData, setPdfDownloadData] = useState({
    link: "",
    name: "",
  });

  function saveByteArray(reportName, byte) {
    var blob = new Blob([byte], { type: "application/pdf" });
    setPdfDownloadData({
      ...pdfDownloadData,
      link: window.URL.createObjectURL(blob),
      name: reportName,
    });
  }

  const [copyText, setCopyText] = useState("Click to Copy");

  const handleModeSwitchButtonClick = () => {
    setTwoPageMode(!twoPageMode)
  };

  const handleClick = () => {
    navigator.clipboard.writeText(window.location.href).then(()=>{
      setCopyText("Copied ✓")
    }).catch(()=>{
      setCopyText("Click to Copy")
    });
  };


  const handlePageClickSingle = (event) => {
    const { nativeEvent } = event;
    const { offsetX, offsetY, target } = nativeEvent;
    const pageWidth = target.clientWidth;

    const fullscreenTouchAreaHeight = 100;
    if (offsetY < fullscreenTouchAreaHeight){
      toggleFullscreen();
    }
    else if (offsetX < pageWidth / 2) {
      changePageIfPossible(-1);
    } else {
      changePageIfPossible(1);
    }
  };


  const handlePageClickLeft = (event) => {
    const { nativeEvent } = event;
    const { offsetX, offsetY, target } = nativeEvent;
    const pageWidth = target.clientWidth;

    const fullscreenTouchAreaHeight = 100;
    if (offsetY < fullscreenTouchAreaHeight){
      toggleFullscreen();
    }
    else if (offsetX < pageWidth / 2) {
      changePageIfPossible(-2);
    } else {
      changePageIfPossible(-1);
    }
  };

  const handlePageClickRight = (event) => {
    const { nativeEvent } = event;
    const { offsetX, offsetY, target } = nativeEvent;
    const pageWidth = target.clientWidth;

    const fullscreenTouchAreaHeight = 100;
    if (offsetY < fullscreenTouchAreaHeight){
      toggleFullscreen();
    }
    else if (offsetX < pageWidth / 2) {
      changePageIfPossible(1);
    } else {
      changePageIfPossible(2);
    }
  };

  // Toggle fullscreen mode
  const toggleFullscreen = () => {
    const elem = document.documentElement;
    if (!document.fullscreenElement) {
      if (elem.requestFullscreen) {
        elem.requestFullscreen();
      } else if (elem.mozRequestFullScreen) { // Firefox
        elem.mozRequestFullScreen();
      } else if (elem.webkitRequestFullscreen) { // Chrome, Safari, and Opera
        elem.webkitRequestFullscreen();
      } else if (elem.msRequestFullscreen) { // IE/Edge
        elem.msRequestFullscreen();
      }
      setIsFullscreen(true);
    } else {
      if (document.exitFullscreen) {
        document.exitFullscreen();
      } else if (document.mozCancelFullScreen) { // Firefox
        document.mozCancelFullScreen();
      } else if (document.webkitExitFullscreen) { // Chrome, Safari, and Opera
        document.webkitExitFullscreen();
      } else if (document.msExitFullscreen) { // IE/Edge
        document.msExitFullscreen();
      }
      setIsFullscreen(false);
    }
  };

  const [editModal, setEditModal] = useState(false);

  const scaleLeft = documentSize.height / pdfDimensionsLeft.height;
  const scaleRight = documentSize.height / pdfDimensionsRight.height;

  return (
    <Fragment>
      {!isFullscreen && <SideBar />}
      <div className="home_content">
        <div className="document_container">
          <div className="doc_wrapper">
            <div className="noselect document" ref={documentRef}>
              <Document
                file={pdf}
                onLoadSuccess={onDocumentLoadSuccess}
              >

                <div className="page-container">
                  {twoPageMode ? (
                    <>
                      <Page
                        pageNumber={pageNumber}
                        scale={scaleLeft}
                        onLoadSuccess={onPdfPageLoadSuccessLeft}
                        onClick={(e) => handlePageClickLeft(e)}
                      />
                      <Page
                        pageNumber={pageNumber + 1}
                        scale={scaleRight}
                        onLoadSuccess={onPdfPageLoadSuccessRight}
                        onClick={(e) => handlePageClickRight(e)}
                      />
                    </>
                  ) : (
                    <Page
                      pageNumber={pageNumber}
                      scale={scaleLeft}
                      onLoadSuccess={onPdfPageLoadSuccessLeft}
                      onClick={(e) => handlePageClickSingle(e)}
                    />
                  )}
                </div>
              </Document>
            </div>
          </div>
        </div>
        {!isFullscreen ?
         <>
           <button className="mode-switch-button" onClick={handleModeSwitchButtonClick}>
             {twoPageMode ? "1 Page" : "2 Pages"}
           </button>
         </>
         : null}
      </div>
    </Fragment>
  );
}

const mapStateToProps = (state) => ({
  sheetPages: state.data.sheetPages,
  composerPages: state.data.composerPages,
  sheets: state.data.sheets,
  composers: state.data.composers,
  sheetPage: state.data.sheetPage,
  totalSheetPages: state.data.totalSheetPages,
  composerPage: state.data.composerPage,
  totalComposerPages: state.data.totalComposerPages,
});

const mapActionsToProps = {
  getSheetPage,
  setSheetPage,
  getComposerPage,
  setComposerPage,
};

export default connect(mapStateToProps, mapActionsToProps)(SheetViewer);
