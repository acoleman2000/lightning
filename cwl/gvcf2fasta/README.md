# GVCF2FASTA

This directory contains cwl files and associated shell scripts for converting gVCF files into FASTA files. 

## Main workflow

The main workflow is `maingvcf2fasta.cwl`.
The main workflow runs gVCF to FASTA workflow with a variety of different inputs/gVCF file formats (split/non-ref/etc).

### Inputs**
VCF files can be provided in many different formats, depending on which version of the workflow is run. 

VCF files must be in `.gz` format.
**VCF TYPES**
* **vcfsdir** - A path to a directory containing VCF files. 
* **vcfs** - An array of VCF files
* **vcftars** - An array of VCF files in tar format
* **splitvcfdirs** - An input directory of split gVCFs.

Additionally, for all workflow subtypes a whole genome BED, reference FASFA, and gqcutoff must be provided.

Simply providing vcf files (in the form of **vcfsdir**) and `.bed`/`.fa` files and gqcutoff are enough to run the most basic version of the workflow.

However, for running more complicated versions of the workflow, additional inputs may be required.

* **sampleids** - sample ids for provided gVCF files. Can be inferred if not provided.
* **chrs** -
* **refsdir** -
* **mapsdir** -
* **panelnocallbed** - 
* **panelcallbed** - 

## Subworkflows

### Scatter files 
Scatter files use CWL scatter functionality to run gvcf2fasta on multiple gvcf file. The scatter workflow ran depends on the input provided. 
### Helpers
Helper files consist of various types of CWL tools. These are used to help the gVCF to FASTA pipeline.
* ***getfiles.cwl*** - This is an ExpressionTool that creates a list of VCFs and sample names given an input directory.
* ***append-sanple-id.cwl*** - This is an ExpressionTool that combines a sampleid amd suffix into one string.
* ***concat-get_bed_varonlyvcf.cwl*** - This is a CommandLineTool that concatenates and gets no call BED and variant only VCF from gVCF. It's used primarily in 
* ***fixvcf-get_bed_varonlyvcf.cwl*** - Fix gVCF and get no call BED and variant only VCF from gVCF
* ***get_bed_varonlyvcf.cwl*** - Get no call BED and variant only VCF from gVCF
* ***untar-concat-get_bed_varonlyvcf.cwl*** - Untar, concatenate, and get no call BED and variant only VCF from gVCF

### gcvf2fasta
These workflows provide the actual functionality of converting from gVCF to FASTA, with various conditions
* ***gvcf2fasta_nonrefvcf-imputation-wf.cwl*** - Impute gVCF and convert to FASTA for gVCF with NON_REF
  * **Helpers used**: `fixvcf-get_bed_varonlyvcf.cwl`, `imputation-wf.cwl`, `bcftools-consensus.cwl` 
* ***gvcf2fasta_nonrefvcf-wf.cwl*** - Convert gVCF to FASTA for gVCF with NON_REF
  * **Helpers used**: `fixvcf-get_bed_varonlyvcf.cwl`, `bcftools-consensus.cwl`
* ***gvcf2fasta_splitvcf-imputation-wf.cwl*** - Impute gVCF and convert to FASTA for gVCF split by chromosome
  * **Helpers used**: `concat-get_bed_varonlyvcf.cwl`, `imputation-wf.cwl`, `append-sampleid.cwl`, `bcftools-consensus.cwl`
* ***gvcf2fasta_splitvcf-wf.cwl*** - Convert gVCF to FASTA for gVCF split by chromosome
  * **Helpers used**: `concat-get_bed_varonlyvcf.cwl`, `bcftools-consensus.cwl`
* ***gvcf2fasta_splitvcftar-wf.cwl*** - Convert gVCF to FASTA for gVCF tar split by chromosome
  * **Helpers used**: `untar-concat-get_bed_varonlyvcf.cwl`, `bcftools-consensus.cwl`
* ***gvcf2fasta-wf.cwl*** - Convert gVCF to FASTA
  * **Helpers used**: `get_bed_varonlyvcf.cwl`, `bcftools-consensus.cwl`

## Shell scripts
* ***bcftools-consensus.sh*** - Runs `bcftools concat` and `bedtools subtract`.
* ***concat-get_bed_varonlyvcf.sh*** - Runs `bcftools view` and `bedtools subtract`.
* ***fixvcf-get_bed_varonlyvcf.sh*** - Runs `bcftools consensus` on vcf file for halotypes 1 and 2.
* ***untar-concat.sh*** - Untars and concatenates using `bcftools`.