.. _smoke-fixture:

Smoke Test Fixture
==================

This fixture exercises the main rst-to-mdx transforms so the smoke
test can detect regressions end to end. Keep it small and intentional;
any change here must be matched by an intentional update to
``smoke.expected.mdx``.

Headings and Inline
-------------------

A short paragraph with **bold**, *italic*, and ``inline code``.
See :ref:`another-anchor` for the cross-reference fallback behavior
(no docs-root is passed, so the label falls back to a TODO marker).

Code Blocks
-----------

.. code-block:: bash

   echo "hello smoke"
   exit 0

Tables
------

.. list-table::
   :header-rows: 1

   * - Column A
     - Column B
   * - one
     - two
   * - three
     - four

Admonitions
-----------

.. note::

   This is a note. It becomes a Mintlify ``<Note>`` component.

Images
------

.. image:: ./logo.png
   :alt: example smoke image
